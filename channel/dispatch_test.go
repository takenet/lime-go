package channel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/address"
)

func TestDispatchClientPreservesSessionAndCallingContext(t *testing.T) {
	transport, peer := newInProcessTransportPair(t.Name(), 4)
	defer silentClose(transport)
	defer silentClose(peer)
	client := &ClientChannel{channel: &channel{
		transport: transport, state: SessionStateEstablished, sessionID: "session",
		localNode: address.ParseNode("app@example/instance"), remoteNode: address.ParseNode("router@example/server"),
	}}
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "caller")
	for _, envelope := range []EnvelopeValue{&Message{}, &Notification{}, &RequestCommand{}} {
		mux := &EnvelopeMux{}
		called := false
		handler := func(actual context.Context) error {
			require.Equal(t, "caller", actual.Value(key{}))
			id, ok := ContextSessionID(actual)
			require.True(t, ok)
			require.Equal(t, client.sessionID, id)
			local, ok := ContextSessionLocalNode(actual)
			require.True(t, ok)
			require.Equal(t, client.localNode, local)
			remote, ok := ContextSessionRemoteNode(actual)
			require.True(t, ok)
			require.Equal(t, client.remoteNode, remote)
			called = true
			return nil
		}
		mux.MessageHandlerFunc(nil, func(ctx context.Context, message *Message, sender Sender) error {
			require.Same(t, envelope, message)
			require.Same(t, client, sender)
			return handler(ctx)
		})
		mux.NotificationHandlerFunc(nil, func(ctx context.Context, notification *Notification) error {
			require.Same(t, envelope, notification)
			return handler(ctx)
		})
		mux.RequestCommandHandlerFunc(nil, func(ctx context.Context, command *RequestCommand, sender Sender) error {
			require.Same(t, envelope, command)
			require.Same(t, client, sender)
			return handler(ctx)
		})
		require.NoError(t, mux.DispatchClient(ctx, client, envelope))
		require.True(t, called, "handler must finish before dispatch returns")
	}
}

func TestDispatchClientRejectsInvalidInputsAndReturnsHandlerError(t *testing.T) {
	mux := &EnvelopeMux{}
	transport, peer := newInProcessTransportPair(t.Name(), 4)
	defer silentClose(transport)
	defer silentClose(peer)
	client := &ClientChannel{channel: &channel{transport: transport, state: SessionStateEstablished}}
	ctx := context.Background()
	//nolint:staticcheck // SA1012: intentionally test nil-context rejection.
	require.Error(t, mux.DispatchClient(nil, client, &Message{}))
	require.Error(t, mux.DispatchClient(ctx, nil, &Message{}))
	require.Error(t, mux.DispatchClient(ctx, client, nil))
	require.Error(t, mux.DispatchClient(ctx, &ClientChannel{}, &Message{}))
	for _, envelope := range []EnvelopeValue{(*Message)(nil), (*Notification)(nil), (*RequestCommand)(nil)} {
		require.Error(t, mux.DispatchClient(ctx, client, envelope))
	}
	require.Error(t, mux.DispatchClient(ctx, client, &ResponseCommand{}))
	require.Error(t, mux.DispatchClient(ctx, &ClientChannel{channel: &channel{state: SessionStateNew}}, &Message{}))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, mux.DispatchClient(canceled, client, &Message{}), context.Canceled)
	failure := errors.New("handler failed")
	mux.MessageHandlerFunc(nil, func(context.Context, *Message, Sender) error { return failure })
	require.ErrorIs(t, mux.DispatchClient(ctx, client, &Message{}), failure)
}
