package channel

import (
	"context"
	"reflect"

	"github.com/takenet/lime-go/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func startHandlerSpan(ctx context.Context, envelope EnvelopeValue, name string) (context.Context, trace.Span) {
	ctx, span := tracing.StartSpan(tracing.ExtractContext(ctx, envelope), envelope, name, trace.WithSpanKind(trace.SpanKindConsumer))
	if node, ok := ContextSessionLocalNode(ctx); ok && node != (Node{}) {
		span.SetAttributes(attribute.String("lime.channel.local_node", node.String()))
	}
	if node, ok := ContextSessionRemoteNode(ctx); ok && node != (Node{}) {
		span.SetAttributes(attribute.String("lime.channel.remote_node", node.String()))
	}
	return ctx, span
}

func setChannelTags(span trace.Span, c *channel) {
	if c.localNode != (Node{}) {
		span.SetAttributes(attribute.String("lime.channel.local_node", c.localNode.String()), attribute.String("lime.node", c.localNode.String()))
	}
	if c.remoteNode != (Node{}) {
		span.SetAttributes(attribute.String("lime.channel.remote_node", c.remoteNode.String()))
	}
	if endpoint := c.transport.LocalAddr(); endpoint != nil {
		span.SetAttributes(attribute.String("lime.transport.local_endpoint", endpoint.String()))
	}
	if endpoint := c.transport.RemoteAddr(); endpoint != nil {
		span.SetAttributes(attribute.String("lime.transport.remote_endpoint", endpoint.String()))
	}
	transportType := reflect.TypeOf(c.transport)
	if transportType.Kind() == reflect.Pointer {
		transportType = transportType.Elem()
	}
	span.SetAttributes(attribute.String("lime.transport.type", transportType.Name()))
}

func finishEnvelopeSpan(span trace.Span, err *error, envelope EnvelopeValue) {
	if *err != nil {
		span.RecordError(*err)
		span.SetStatus(codes.Error, (*err).Error())
	} else {
		tracing.SetEnvelopeTags(span, envelope)
	}
	span.End()
}
