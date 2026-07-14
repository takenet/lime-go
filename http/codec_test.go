package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/takenet/lime-go"
)

func TestDetectEnvelopeKind(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    envelopeKind
		wantErr bool
	}{
		{"message", `{"type":"text/plain","content":"hi"}`, envelopeKindMessage, false},
		{"notification", `{"event":"received"}`, envelopeKindNotification, false},
		{"request command", `{"method":"get","uri":"/account"}`, envelopeKindRequestCommand, false},
		{"response command", `{"method":"get","status":"success"}`, envelopeKindResponseCommand, false},
		{"unknown", `{"foo":"bar"}`, "", true},
		{"invalid json", `not json`, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := detectEnvelopeKind([]byte(tt.json))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

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
