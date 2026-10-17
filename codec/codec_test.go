package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/takenet/lime-go/document"
	"github.com/takenet/lime-go/protocol"
)

func TestRawDocumentEnvelopeRoundTrip(t *testing.T) {
	mediaType := document.MediaType{Type: "application", Subtype: "vnd.example.raw", Suffix: "json"}
	raw := &document.RawDocument{
		Type:  mediaType,
		Value: []byte(`{"id":9007199254740993}`),
	}
	message := &protocol.Message{}
	message.SetContent(raw)

	encoded, err := MarshalEnvelope(message)
	if !assert.NoError(t, err) {
		return
	}
	envelope, err := UnmarshalEnvelope(encoded)
	if !assert.NoError(t, err) {
		return
	}
	decoded, ok := envelope.(*protocol.Message)
	if !assert.True(t, ok) {
		return
	}
	decodedRaw, ok := decoded.Content.(*document.RawDocument)
	if !assert.True(t, ok) {
		return
	}
	assert.Equal(t, mediaType, decodedRaw.MediaType())
	assert.Equal(t, raw.Value, decodedRaw.Value)

	var value struct {
		ID int64 `json:"id"`
	}
	assert.NoError(t, DecodeDocument(decodedRaw, &value))
	assert.Equal(t, int64(9007199254740993), value.ID)
}
