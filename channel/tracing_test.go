package channel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func tracingRecorder(t *testing.T) *tracetest.SpanRecorder {
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

func TestChannelSendsPropagateContext(t *testing.T) {
	recorder := tracingRecorder(t)
	ctx, span := otel.Tracer("test").Start(context.Background(), "send")
	defer span.End()
	for _, preserve := range []bool{false, true} {
		for _, test := range []struct {
			name string
			send func(*channel, context.Context) (EnvelopeValue, error)
		}{
			{"message", func(c *channel, ctx context.Context) (EnvelopeValue, error) {
				message := createMessage()
				if preserve {
					message.Metadata = map[string]string{tracing.TraceParent: "existing", tracing.TraceState: "vendor=value"}
				}
				return message, c.SendMessage(ctx, message)
			}},
			{"notification", func(c *channel, ctx context.Context) (EnvelopeValue, error) {
				notification := createNotification()
				return notification, c.SendNotification(ctx, notification)
			}},
			{"request", func(c *channel, ctx context.Context) (EnvelopeValue, error) {
				command := createGetPingCommand()
				return command, c.SendRequestCommand(ctx, command)
			}},
			{"response", func(c *channel, ctx context.Context) (EnvelopeValue, error) {
				command := createResponseCommand()
				return command, c.SendResponseCommand(ctx, command)
			}},
			{"session", func(c *channel, ctx context.Context) (EnvelopeValue, error) {
				session := &Session{State: SessionStateEstablished}
				return session, c.sendSession(ctx, session)
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				local, remote := newInProcessTransportPair("localhost", 1)
				c := newChannel(local, 1)
				defer silentClose(c)
				c.setState(SessionStateEstablished)
				value, err := test.send(c, ctx)
				require.NoError(t, err)
				received, err := remote.Receive(ctx)
				require.NoError(t, err)
				require.Equal(t, value, received)
				if test.name == "message" && preserve {
					message := received.(*Message)
					require.Equal(t, "existing", message.Metadata[tracing.TraceParent])
					require.Equal(t, "vendor=value", message.Metadata[tracing.TraceState])
				} else {
					extracted := trace.SpanContextFromContext(tracing.ExtractContext(context.Background(), received))
					require.Equal(t, span.SpanContext().TraceID(), extracted.TraceID())
					ended := recorder.Ended()[len(recorder.Ended())-1]
					require.Equal(t, span.SpanContext().SpanID(), ended.Parent().SpanID())
					require.Equal(t, ended.SpanContext().SpanID(), extracted.SpanID())
				}
			})
		}
	}
}

func TestMuxRestoresRemoteParentAndRecordsErrors(t *testing.T) {
	for _, kind := range []string{"message", "notification", "request", "response"} {
		t.Run(kind, func(t *testing.T) {
			recorder := tracingRecorder(t)
			remoteCtx, remoteSpan := otel.Tracer("test").Start(context.Background(), "remote")
			defer remoteSpan.End()
			ctx, localSpan := otel.Tracer("test").Start(context.Background(), "local")
			defer localSpan.End()
			c := &channel{localNode: protocol.ParseNode("local@example.com"), remoteNode: protocol.ParseNode("remote@example.com")}
			ctx = sessionContext(ctx, c)
			expectedError := errors.New("handler failed")
			var handledContext context.Context
			handle := func(ctx context.Context) error {
				handledContext = ctx
				return expectedError
			}
			mux := &EnvelopeMux{}
			var err error
			switch kind {
			case "message":
				message := createMessage()
				tracing.InjectContext(remoteCtx, message)
				mux.MessageHandlerFunc(nil, func(ctx context.Context, _ *Message, _ Sender) error { return handle(ctx) })
				err = mux.handleMessage(ctx, message, nil)
			case "notification":
				notification := createNotification()
				tracing.InjectContext(remoteCtx, notification)
				mux.NotificationHandlerFunc(nil, func(ctx context.Context, _ *Notification) error { return handle(ctx) })
				err = mux.handleNotification(ctx, notification)
			case "request":
				command := createGetPingCommand()
				command.SetURIString("/test")
				tracing.InjectContext(remoteCtx, command)
				mux.RequestCommandHandlerFunc(nil, func(ctx context.Context, _ *RequestCommand, _ Sender) error { return handle(ctx) })
				err = mux.handleRequestCommand(ctx, command, nil)
			case "response":
				command := createResponseCommand()
				tracing.InjectContext(remoteCtx, command)
				mux.ResponseCommandHandlerFunc(nil, func(ctx context.Context, _ *ResponseCommand, _ Sender) error { return handle(ctx) })
				err = mux.handleResponseCommand(ctx, command, nil)
			}
			require.ErrorIs(t, err, expectedError)
			require.Len(t, recorder.Ended(), 1)
			ended := recorder.Ended()[0]
			require.Equal(t, remoteSpan.SpanContext().SpanID(), ended.Parent().SpanID())
			require.True(t, ended.Parent().IsRemote())
			require.Equal(t, trace.SpanKindConsumer, ended.SpanKind())
			require.Equal(t, codes.Error, ended.Status().Code)
			require.Len(t, ended.Events(), 1)
			attributes := make(map[string]string)
			for _, item := range ended.Attributes() {
				attributes[string(item.Key)] = item.Value.AsString()
			}
			require.Equal(t, "local@example.com", attributes["lime.channel.local_node"])
			require.Equal(t, "remote@example.com", attributes["lime.channel.remote_node"])
			reply := createNotification()
			tracing.InjectContextIfAbsent(handledContext, reply)
			require.Equal(t, ended.SpanContext().SpanID(), trace.SpanContextFromContext(tracing.ExtractContext(context.Background(), reply)).SpanID())
		})
	}
}

func TestSessionTracingPreservesCallerAndIsolatesEnvelopes(t *testing.T) {
	recorder := tracingRecorder(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	callerCtx, caller := otel.Tracer("test").Start(ctx, "caller")
	defer caller.End()
	local, remote := newInProcessTransportPair("localhost", 4)
	client := NewClientChannel(local, 4)
	defer silentClose(client)
	server := NewServerChannel(remote, 4, protocol.ParseNode("postmaster@example.com/server"), "session")
	defer silentClose(server)
	serverDone := make(chan error, 1)
	authContext := make(chan trace.SpanContext, 1)
	go func() {
		serverDone <- server.EstablishSession(ctx,
			[]SessionCompression{SessionCompressionNone}, []SessionEncryption{SessionEncryptionNone},
			[]AuthenticationScheme{AuthenticationSchemeGuest},
			func(ctx context.Context, _ Identity, _ Authentication) (*AuthenticationResult, error) {
				authContext <- trace.SpanContextFromContext(ctx)
				return MemberAuthenticationResult(), nil
			},
			func(_ context.Context, node Node, _ *ServerChannel) (Node, error) { return node, nil })
	}()
	session, err := client.EstablishSession(callerCtx, NoneCompressionSelector, NoneEncryptionSelector,
		Identity{Name: "client", Domain: "example.com"}, GuestAuthenticator, "instance")
	require.NoError(t, err)
	require.Equal(t, SessionStateEstablished, session.State)
	require.NoError(t, <-serverDone)
	require.Equal(t, caller.SpanContext().TraceID(), (<-authContext).TraceID())
	require.GreaterOrEqual(t, len(recorder.Ended()), 6)
	for _, span := range recorder.Ended() {
		require.Equal(t, caller.SpanContext().TraceID(), span.SpanContext().TraceID())
		if span.Name() == "ClientChannel.EstablishSession" {
			require.Equal(t, caller.SpanContext().SpanID(), span.Parent().SpanID())
		}
	}
	require.Equal(t, caller.SpanContext().TraceID(), trace.SpanContextFromContext(tracing.ExtractContext(ctx, session)).TraceID())

	var previous trace.TraceID
	for range 2 {
		message := createMessage()
		require.NoError(t, client.SendMessage(ctx, message))
		var received *Message
		select {
		case received = <-server.MsgChan():
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		parent := trace.SpanContextFromContext(tracing.ExtractContext(ctx, received))
		require.True(t, parent.IsValid())
		require.NotEqual(t, caller.SpanContext().TraceID(), parent.TraceID())
		require.NotEqual(t, previous, parent.TraceID())
		previous = parent.TraceID()
	}
}

func TestHandlerTracingIndependentRootsAndHTTPFallback(t *testing.T) {
	recorder := tracingRecorder(t)
	var previous trace.TraceID
	for _, metadata := range []map[string]string{nil, {tracing.TraceParent: "invalid"}} {
		message := createMessage()
		message.Metadata = metadata
		_, span := startHandlerSpan(context.Background(), message, "receive")
		require.True(t, span.SpanContext().IsValid())
		require.NotEqual(t, previous, span.SpanContext().TraceID())
		previous = span.SpanContext().TraceID()
		span.End()
	}
	ctx, parent := otel.Tracer("test").Start(context.Background(), "http")
	defer parent.End()
	_, span := startHandlerSpan(ctx, createMessage(), "receive")
	span.End()
	ended := recorder.Ended()[2]
	require.Equal(t, parent.SpanContext().SpanID(), ended.Parent().SpanID())
}

func TestProcessCommandTracing(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			recorder := tracingRecorder(t)
			local, remote := newInProcessTransportPair("localhost", 1)
			c := newChannel(local, 1)
			defer silentClose(c)
			c.localNode = protocol.ParseNode("local@example.com")
			c.remoteNode = protocol.ParseNode("remote@example.com")
			c.setState(SessionStateEstablished)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			request := createGetPingCommand()
			request.SetURIString("/test")
			remoteDone := make(chan error, 1)
			go func() {
				value, err := remote.Receive(ctx)
				if err != nil {
					remoteDone <- err
					return
				}
				command := value.(*RequestCommand)
				response := command.SuccessResponse()
				if fail {
					response = command.FailureResponse(&Reason{Code: 42, Description: "failed"})
				}
				remoteDone <- remote.Send(ctx, response)
			}()
			_, err := c.ProcessCommand(ctx, request)
			require.NoError(t, err)
			require.NoError(t, <-remoteDone)
			require.Len(t, recorder.Ended(), 2)
			ended := recorder.Ended()[1]
			require.Equal(t, trace.SpanKindClient, ended.SpanKind())
			require.Equal(t, ended.SpanContext().SpanID(), trace.SpanContextFromContext(tracing.ExtractContext(ctx, request)).SpanID())
			if fail {
				require.Equal(t, codes.Error, ended.Status().Code)
			} else {
				require.Equal(t, codes.Ok, ended.Status().Code)
			}
		})
	}
}
