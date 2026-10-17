// Package codec provides the serialization boundary for LIME envelopes and documents.
package codec

import (
	"encoding/json"
	"errors"

	"github.com/takenet/lime-go/document"
	"github.com/takenet/lime-go/protocol"
)

func MarshalEnvelope(envelope protocol.EnvelopeValue) ([]byte, error) {
	return json.Marshal(envelope)
}

func UnmarshalEnvelope(data []byte) (protocol.EnvelopeValue, error) {
	return protocol.UnmarshalEnvelope(data)
}

func MarshalDocument(value document.Document) ([]byte, error) {
	return json.Marshal(value)
}

func UnmarshalDocument(data []byte, mediaType document.MediaType) (document.Document, error) {
	raw := json.RawMessage(append([]byte(nil), data...))
	return document.UnmarshalDocument(&raw, mediaType)
}

func DecodeDocument(value document.Document, destination any) error {
	if value == nil {
		return errors.New("document is nil")
	}
	if destination == nil {
		return errors.New("document destination is nil")
	}

	var data []byte
	if raw, ok := value.(*document.RawDocument); ok {
		data = raw.Value
	} else {
		encoded, err := json.Marshal(value)
		if err != nil {
			return err
		}
		data = encoded
	}
	return json.Unmarshal(data, destination)
}
