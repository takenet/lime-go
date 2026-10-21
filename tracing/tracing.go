// Package tracing provides OpenTelemetry helpers compatible with Lime.Protocol tracing.
// Applications own provider/exporter configuration and must end spans they start.
package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/takenet/lime-go/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

const (
	InstrumentationName = "Lime.Protocol"
	TraceParent         = "traceparent"
	TraceState          = "tracestate"
)

func envelopeOf(value protocol.EnvelopeValue) *protocol.Envelope {
	switch envelope := value.(type) {
	case *protocol.Message:
		if envelope != nil {
			return &envelope.Envelope
		}
	case *protocol.Notification:
		if envelope != nil {
			return &envelope.Envelope
		}
	case *protocol.RequestCommand:
		if envelope != nil {
			return &envelope.Envelope
		}
	case *protocol.ResponseCommand:
		if envelope != nil {
			return &envelope.Envelope
		}
	case *protocol.Session:
		if envelope != nil {
			return &envelope.Envelope
		}
	}
	return nil
}

// ExtractContext restores a valid remote W3C parent while retaining cancellation,
// deadlines and values. Missing or invalid metadata leaves ctx unchanged.
func ExtractContext(ctx context.Context, value protocol.EnvelopeValue) context.Context {
	if envelope := envelopeOf(value); envelope != nil {
		return propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier(envelope.Metadata))
	}
	return ctx
}

// InjectContext replaces the envelope's W3C context and removes stale tracestate.
// An invalid span context leaves the envelope untouched, including nil metadata.
func InjectContext(ctx context.Context, value protocol.EnvelopeValue) {
	envelope := envelopeOf(value)
	if envelope == nil || !trace.SpanContextFromContext(ctx).IsValid() {
		return
	}
	if envelope.Metadata == nil {
		envelope.Metadata = make(map[string]string)
	}
	delete(envelope.Metadata, TraceState)
	propagation.TraceContext{}.Inject(ctx, propagation.MapCarrier(envelope.Metadata))
}

// InjectContextIfAbsent preserves an existing traceparent, matching Lime.Protocol.
func InjectContextIfAbsent(ctx context.Context, value protocol.EnvelopeValue) {
	if envelope := envelopeOf(value); envelope != nil {
		if _, exists := envelope.Metadata[TraceParent]; !exists {
			InjectContext(ctx, value)
		}
	}
}

// CopyContext copies only W3C trace fields, leaving other metadata untouched.
func CopyContext(from, to protocol.EnvelopeValue) {
	source, target := envelopeOf(from), envelopeOf(to)
	if source == nil || target == nil {
		return
	}
	parent, exists := source.Metadata[TraceParent]
	if !exists {
		return
	}
	if target.Metadata == nil {
		target.Metadata = make(map[string]string)
	}
	target.Metadata[TraceParent] = parent
	if state := source.Metadata[TraceState]; strings.TrimSpace(state) != "" {
		target.Metadata[TraceState] = state
	} else {
		delete(target.Metadata, TraceState)
	}
}

// StartSpan starts an envelope-aware span, preferring ctx's active parent over
// metadata. To prioritize metadata, pass ExtractContext(ctx, envelope) as ctx.
// It sets Lime.Protocol tags and injects the new context into the envelope.
// Status is left unset until processing finishes, so handler and I/O errors
// take precedence over the result carried by the envelope.
// Nil envelopes and ping commands return a no-op span and do not mutate metadata.
func StartSpan(ctx context.Context, value protocol.EnvelopeValue, name string, options ...trace.SpanStartOption) (context.Context, trace.Span) {
	if envelopeOf(value) == nil || isPing(value) {
		_, span := noop.NewTracerProvider().Tracer(InstrumentationName).Start(ctx, name)
		return ctx, span
	}
	if !trace.SpanContextFromContext(ctx).IsValid() {
		ctx = ExtractContext(ctx, value)
	}
	ctx, span := otel.Tracer(InstrumentationName).Start(ctx, name, options...)
	setEnvelopeTags(span, value, false)
	InjectContext(ctx, value)
	return ctx, span
}

func isPing(value protocol.EnvelopeValue) bool {
	switch command := value.(type) {
	case *protocol.RequestCommand:
		return command != nil && ((command.URI != nil && command.URI.String() == "/ping") || isPingType(command.Type))
	case *protocol.ResponseCommand:
		return command != nil && isPingType(command.Type)
	}
	return false
}

func isPingType(mediaType *protocol.MediaType) bool {
	return mediaType != nil && mediaType.String() == "application/vnd.lime.ping+json"
}

func enumName(value string) string {
	if value == "" {
		return ""
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

// SetEnvelopeTags records envelope attributes and the protocol operation result.
// Expected command rejections leave status unset; unknown failures remain errors.
func SetEnvelopeTags(span trace.Span, value protocol.EnvelopeValue) {
	setEnvelopeTags(span, value, true)
}

// SetEnvelopeAttributes records envelope attributes without changing span status.
// Use it when processing or transmitting an envelope, rather than executing it.
func SetEnvelopeAttributes(span trace.Span, value protocol.EnvelopeValue) {
	setEnvelopeTags(span, value, false)
}

func setEnvelopeTags(span trace.Span, value protocol.EnvelopeValue, completed bool) {
	envelope := envelopeOf(value)
	if span == nil || envelope == nil {
		return
	}
	var envelopeType string
	switch typed := value.(type) {
	case *protocol.Message:
		envelopeType = "Message"
		span.SetAttributes(attribute.String("lime.message.type", typed.Type.String()))
	case *protocol.Notification:
		envelopeType = "Notification"
		span.SetAttributes(attribute.String("lime.notification.event", enumName(string(typed.Event))))
		setStatus(span, typed.Event == protocol.NotificationEventFailed, !completed, typed.Reason)
	case *protocol.Session:
		envelopeType = "Session"
		state := enumName(string(typed.State))
		if state == "" {
			state = "New"
		}
		span.SetAttributes(attribute.String("lime.session.state", state))
		setStatus(span, typed.State == protocol.SessionStateFailed, !completed || state == "New", typed.Reason)
	case *protocol.RequestCommand:
		envelopeType = "Command"
		setCommandTags(span, &typed.Command, "Pending")
		if typed.URI != nil {
			span.SetAttributes(attribute.String("lime.command.uri", typed.URI.String()))
		}
	case *protocol.ResponseCommand:
		envelopeType = "Command"
		status := enumName(string(typed.Status))
		if status == "" {
			status = "Pending"
		}
		setCommandTags(span, &typed.Command, status)
		if typed.Reason != nil {
			span.SetAttributes(attribute.Int("lime.command.reason.code", typed.Reason.Code))
		}
		if typed.Status == protocol.CommandStatusFailure {
			outcome := commandFailureOutcome(typed.Reason)
			span.SetAttributes(attribute.String("lime.command.outcome", outcome))
			if outcome == "error" {
				setStatus(span, true, !completed, typed.Reason)
			}
		} else {
			span.SetAttributes(attribute.String("lime.command.outcome", strings.ToLower(status)))
			setStatus(span, false, !completed || status == "Pending", typed.Reason)
		}
	}
	span.SetAttributes(attribute.String("lime.envelope.type", envelopeType), attribute.String("lime.envelope.id", envelope.ID))
	if envelope.From != (protocol.Node{}) {
		span.SetAttributes(attribute.String("lime.envelope.from", envelope.From.String()))
	}
	if envelope.To != (protocol.Node{}) {
		span.SetAttributes(attribute.String("lime.envelope.to", envelope.To.String()))
	}
}

func setCommandTags(span trace.Span, command *protocol.Command, status string) {
	method := enumName(string(command.Method))
	if method == "" {
		method = "Get"
	}
	span.SetAttributes(attribute.String("lime.command.method", method), attribute.String("lime.command.status", status))
	if command.Type != nil {
		span.SetAttributes(attribute.String("lime.command.type", command.Type.String()))
	}
}

func commandFailureOutcome(reason *protocol.Reason) string {
	if reason != nil {
		switch reason.Code {
		case protocol.ReasonCodeCommandResourceNotFound:
			return "not_found"
		case protocol.ReasonCodeCommandInvalidArgument:
			return "invalid"
		case protocol.ReasonCodeCommandNotAllowed:
			return "forbidden"
		case protocol.ReasonCodeCommandResourceNotSupported:
			return "unsupported_resource"
		case protocol.ReasonCodeCommandMethodNotSupported:
			return "unsupported_method"
		}
	}
	return "error"
}

func setStatus(span trace.Span, failed, pending bool, reason *protocol.Reason) {
	if pending {
		return
	}
	if failed {
		description := "Code : "
		if reason != nil {
			description = fmt.Sprintf("Code %d: %s", reason.Code, reason.Description)
		}
		span.SetStatus(codes.Error, description)
	} else {
		span.SetStatus(codes.Ok, "")
	}
}
