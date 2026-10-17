package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lime "github.com/takenet/lime-go/protocol"
)

func TestDecodeEnvelope(t *testing.T) {
	msgJSON := `{"type":"text/plain","content":"hi"}`
	env, err := decodeEnvelope([]byte(msgJSON))
	require.NoError(t, err)
	msg, ok := env.(*lime.Message)
	require.True(t, ok)
	text := lime.TextDocument("hi")
	assert.Equal(t, &text, msg.Content)

	notJSON := `{"event":"received"}`
	env, err = decodeEnvelope([]byte(notJSON))
	require.NoError(t, err)
	not, ok := env.(*lime.Notification)
	require.True(t, ok)
	assert.Equal(t, lime.NotificationEventReceived, not.Event)

	requestJSON := `{"method":"get","uri":"/account"}`
	env, err = decodeEnvelope([]byte(requestJSON))
	require.NoError(t, err)
	_, ok = env.(*lime.RequestCommand)
	require.True(t, ok)

	responseJSON := `{"method":"get","status":"success"}`
	env, err = decodeEnvelope([]byte(responseJSON))
	require.NoError(t, err)
	_, ok = env.(*lime.ResponseCommand)
	require.True(t, ok)
}

func TestDecodeEnvelope_WhenUnknownKind_ReturnsError(t *testing.T) {
	_, err := decodeEnvelope([]byte(`{"foo":"bar"}`))
	assert.Error(t, err)
}

func TestEnvelopeContentType(t *testing.T) {
	tests := []struct {
		name string
		env  any
		want string
	}{
		{"message", &lime.Message{}, "application/vnd.lime.message+json"},
		{"notification", &lime.Notification{}, "application/vnd.lime.notification+json"},
		{"request command", &lime.RequestCommand{}, "application/vnd.lime.command+json"},
		{"response command", &lime.ResponseCommand{}, "application/vnd.lime.command+json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := envelopeContentType(tt.env)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEnvelopeContentType_WhenUnsupportedType_ReturnsError(t *testing.T) {
	_, err := envelopeContentType("not an envelope")
	assert.Error(t, err)
}
