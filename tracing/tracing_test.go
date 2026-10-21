package tracing

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const parentHeader = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"

func provider(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	return recorder
}

func TestStartSpanParent(t *testing.T) {
	for _, test := range []struct {
		name               string
		active, prioritize bool
	}{
		{name: "remote parent"},
		{name: "active parent", active: true},
		{name: "prioritized remote parent", active: true, prioritize: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := provider(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			message := &protocol.Message{Envelope: protocol.Envelope{Metadata: map[string]string{TraceParent: parentHeader, TraceState: "vendor=value"}}}
			remote := trace.SpanContextFromContext(ExtractContext(ctx, message))
			expected := remote
			if test.active {
				var parent trace.Span
				ctx, parent = otel.Tracer("test").Start(ctx, "parent")
				defer parent.End()
				expected = parent.SpanContext()
			}
			if test.prioritize {
				ctx = ExtractContext(ctx, message)
				expected = remote
			}
			ctx, span := StartSpan(ctx, message, "process", trace.WithSpanKind(trace.SpanKindConsumer))
			span.End()
			ended := recorder.Ended()[0]
			require.Equal(t, expected, ended.Parent())
			require.Equal(t, "Lime.Protocol", ended.InstrumentationScope().Name)
			require.Equal(t, trace.SpanKindConsumer, ended.SpanKind())
			require.Equal(t, span.SpanContext().TraceID(), trace.SpanContextFromContext(ExtractContext(context.Background(), message)).TraceID())
			require.Equal(t, span.SpanContext().SpanID(), trace.SpanContextFromContext(ExtractContext(context.Background(), message)).SpanID())
			cancel()
			require.ErrorIs(t, ctx.Err(), context.Canceled)
		})
	}
}

func TestEnvelopeTags(t *testing.T) {
	mediaType := protocol.MediaTypeTextPlain()
	request := &protocol.RequestCommand{Command: protocol.Command{Method: protocol.CommandMethodGet, Type: &mediaType}}
	request.SetURIString("/test")
	for _, test := range []struct {
		name       string
		envelope   protocol.EnvelopeValue
		attributes map[string]string
		status     codes.Code
	}{
		{"message", &protocol.Message{Type: mediaType}, map[string]string{"lime.envelope.type": "Message", "lime.message.type": "text/plain"}, codes.Unset},
		{"request", request, map[string]string{"lime.envelope.type": "Command", "lime.command.method": "Get", "lime.command.status": "Pending", "lime.command.type": "text/plain", "lime.command.uri": "/test"}, codes.Unset},
		{"success", &protocol.ResponseCommand{Status: protocol.CommandStatusSuccess}, map[string]string{"lime.command.status": "Success"}, codes.Ok},
		{"failure", &protocol.ResponseCommand{Status: protocol.CommandStatusFailure, Reason: &protocol.Reason{Code: 42, Description: "failed"}}, map[string]string{"lime.command.status": "Failure"}, codes.Error},
		{"notification", &protocol.Notification{Event: protocol.NotificationEventReceived}, map[string]string{"lime.notification.event": "Received"}, codes.Ok},
		{"failed notification", &protocol.Notification{Event: protocol.NotificationEventFailed}, map[string]string{"lime.notification.event": "Failed"}, codes.Error},
		{"session", &protocol.Session{State: protocol.SessionStateEstablished}, map[string]string{"lime.session.state": "Established"}, codes.Ok},
		{"failed session", &protocol.Session{State: protocol.SessionStateFailed}, map[string]string{"lime.session.state": "Failed"}, codes.Error},
		{"new session", &protocol.Session{}, map[string]string{"lime.session.state": "New"}, codes.Unset},
		{"pending response", &protocol.ResponseCommand{}, map[string]string{"lime.command.status": "Pending"}, codes.Unset},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := provider(t)
			envelope := envelopeOf(test.envelope)
			envelope.ID = "id"
			envelope.From = protocol.ParseNode("from@example.com")
			envelope.To = protocol.ParseNode("to@example.com")
			_, span := StartSpan(context.Background(), test.envelope, "test")
			SetEnvelopeTags(span, test.envelope)
			span.End()
			ended := recorder.Ended()[0]
			attributes := make(map[string]string)
			for _, item := range ended.Attributes() {
				attributes[string(item.Key)] = item.Value.AsString()
			}
			for key, value := range test.attributes {
				require.Equal(t, value, attributes[key], key)
			}
			require.Equal(t, "id", attributes["lime.envelope.id"])
			require.Equal(t, "from@example.com", attributes["lime.envelope.from"])
			require.Equal(t, "to@example.com", attributes["lime.envelope.to"])
			require.Equal(t, test.status, ended.Status().Code)
			if test.name == "failure" {
				require.Equal(t, "Code 42: failed", ended.Status().Description)
			}
		})
	}
}

func TestCommandFailureClassification(t *testing.T) {
	for _, test := range []struct {
		name    string
		reason  *protocol.Reason
		outcome string
		status  codes.Code
	}{
		{"not found", &protocol.Reason{Code: protocol.ReasonCodeCommandResourceNotFound}, "not_found", codes.Unset},
		{"invalid", &protocol.Reason{Code: protocol.ReasonCodeCommandInvalidArgument}, "invalid", codes.Unset},
		{"forbidden", &protocol.Reason{Code: protocol.ReasonCodeCommandNotAllowed}, "forbidden", codes.Unset},
		{"unsupported resource", &protocol.Reason{Code: protocol.ReasonCodeCommandResourceNotSupported}, "unsupported_resource", codes.Unset},
		{"unsupported method", &protocol.Reason{Code: protocol.ReasonCodeCommandMethodNotSupported}, "unsupported_method", codes.Unset},
		{"processing error", &protocol.Reason{Code: protocol.ReasonCodeCommandProcessingError}, "error", codes.Error},
		{"unknown failure", &protocol.Reason{Code: 42}, "error", codes.Error},
		{"missing reason", nil, "error", codes.Error},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := provider(t)
			response := &protocol.ResponseCommand{Status: protocol.CommandStatusFailure, Reason: test.reason}
			_, span := StartSpan(context.Background(), response, "execute")
			require.Equal(t, codes.Unset, span.(sdktrace.ReadWriteSpan).Status().Code)
			SetEnvelopeTags(span, response)
			span.End()
			ended := recorder.Ended()[0]
			require.Equal(t, test.status, ended.Status().Code)
			require.Contains(t, ended.Attributes(), attribute.String("lime.command.status", "Failure"))
			require.Contains(t, ended.Attributes(), attribute.String("lime.command.outcome", test.outcome))
			if test.reason != nil {
				require.Contains(t, ended.Attributes(), attribute.Int("lime.command.reason.code", test.reason.Code))
			}
			_, transportSpan := otel.Tracer("test").Start(context.Background(), "send")
			SetEnvelopeAttributes(transportSpan, response)
			transportSpan.End()
			require.Equal(t, codes.Unset, recorder.Ended()[1].Status().Code)
		})
	}
}

func TestContextPropagation(t *testing.T) {
	provider(t)
	ctx, span := otel.Tracer("test").Start(context.Background(), "parent")
	defer span.End()
	message := &protocol.Message{Envelope: protocol.Envelope{Metadata: map[string]string{"custom": "keep", TraceState: "stale=value"}}}
	InjectContext(ctx, message)
	require.NotContains(t, message.Metadata, TraceState)
	require.Equal(t, "keep", message.Metadata["custom"])
	parent := message.Metadata[TraceParent]
	message.Metadata[TraceParent] = parentHeader
	InjectContextIfAbsent(ctx, message)
	require.Equal(t, parentHeader, message.Metadata[TraceParent])
	CopyContext(message, &protocol.Notification{})
	target := &protocol.Notification{Envelope: protocol.Envelope{Metadata: map[string]string{TraceState: "stale=value"}}}
	CopyContext(message, target)
	require.Equal(t, parentHeader, target.Metadata[TraceParent])
	require.NotContains(t, target.Metadata, TraceState)
	message.Metadata[TraceState] = "vendor=value"
	CopyContext(message, target)
	require.Equal(t, "vendor=value", target.Metadata[TraceState])
	message.Metadata[TraceParent] = "invalid"
	require.Equal(t, span.SpanContext(), trace.SpanContextFromContext(ExtractContext(ctx, message)))
	InjectContext(context.Background(), message)
	require.Equal(t, "invalid", message.Metadata[TraceParent])
	message.Metadata = nil
	InjectContextIfAbsent(ctx, message)
	require.Equal(t, parent, message.Metadata[TraceParent])
}

func TestNoopAndNilSafety(t *testing.T) {
	recorder := provider(t)
	ping := &protocol.RequestCommand{}
	ping.SetURIString("/ping")
	pingType, err := protocol.ParseMediaType("application/vnd.lime.ping+json")
	require.NoError(t, err)
	for _, value := range []protocol.EnvelopeValue{nil, (*protocol.Message)(nil), (*protocol.Notification)(nil), (*protocol.RequestCommand)(nil), (*protocol.ResponseCommand)(nil), (*protocol.Session)(nil), ping, &protocol.ResponseCommand{Command: protocol.Command{Type: &pingType}}} {
		ctx, span := StartSpan(context.Background(), value, "ignored")
		span.End()
		InjectContext(ctx, value)
		InjectContextIfAbsent(ctx, value)
		CopyContext(value, nil)
		SetEnvelopeTags(span, value)
		SetEnvelopeTags(nil, value)
		SetEnvelopeAttributes(span, value)
		SetEnvelopeAttributes(nil, value)
		require.False(t, span.IsRecording())
	}
	require.Empty(t, recorder.Ended())
	require.Nil(t, ping.Metadata)
}
