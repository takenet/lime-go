package lime

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Provides the transport of a content between nodes in a network.
type Message struct {
	Envelope
	// MIME declaration of the content type of the message.
	Type MediaType `json:"type"`
	// Message body content
	Content Document `json:"content"`
}

func (m *Message) SetContent(d Document) {
	m.Content = d
	m.Type = d.GetMediaType()
}

func (m *Message) UnmarshalJSON(b []byte) error {
	var messageMap map[string]json.RawMessage
	err := json.Unmarshal(b, &messageMap)
	if err != nil {
		return err
	}
	message := Message{}

	for k, v := range messageMap {
		var ok bool
		ok, err = message.Envelope.unmarshalJSONField(k, v)
		if !ok {
			ok, err = message.unmarshalJSONField(k, v)
		}

		if !ok {
			return fmt.Errorf(`unknown message field '%v'`, k)
		}

		if err != nil {
			return err
		}
	}

	// Handle the content
	v, ok := messageMap["content"]
	if !ok {
		return errors.New("content is required")
	}
	if message.Type == (MediaType{}) {
		return errors.New("type is required")
	}

	factory, err := GetDocumentFactory(message.Type)
	if err != nil {
		return err
	}

	// Create the document type instance and unmarshal the json to it
	document := factory()
	err = json.Unmarshal(v, &document)
	if err != nil {
		return err
	}
	message.Content = document

	*m = message
	return nil
}

func (m *Message) unmarshalJSONField(n string, v json.RawMessage) (bool, error) {
	switch n {
	case "type":
		err := json.Unmarshal(v, &m.Type)
		return true, err
	case "content":
		return true, nil // Handled externally
	}
	return false, nil
}
