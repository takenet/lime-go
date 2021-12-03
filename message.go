package lime

import (
	"encoding/json"
	"errors"
)

// Message Provides the transport of a Content between nodes in a network.
type Message struct {
	EnvelopeBase

	// Type MIME declaration of the Content type of the message.
	Type MediaType `json:"type"`

	// Content Message body content
	Content Document `json:"content"`
}

func (m *Message) SetContent(d Document) {
	m.Content = d
	m.Type = d.GetMediaType()
}

func (m *Message) MarshalJSON() ([]byte, error) {
	raw, err := m.toRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (m *Message) UnmarshalJSON(b []byte) error {
	raw := rawEnvelope{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	message := Message{}
	err = message.populate(&raw)
	if err != nil {
		return err
	}

	*m = message
	return nil
}

func (m *Message) toRawEnvelope() (*rawEnvelope, error) {
	raw, err := m.EnvelopeBase.toRawEnvelope()
	if err != nil {
		return nil, err
	}

	if m.Content == nil {
		return nil, errors.New("message content is required")
	}
	b, err := json.Marshal(m.Content)
	if err != nil {
		return nil, err
	}
	content := json.RawMessage(b)

	raw.Type = &m.Type
	raw.Content = &content

	return raw, nil
}

func (m *Message) populate(raw *rawEnvelope) error {
	err := m.EnvelopeBase.populate(raw)
	if err != nil {
		return err
	}

	// Create the document type instance and unmarshal the json To it
	if raw.Type == nil {
		return errors.New("message type is required")
	}

	if raw.Content == nil {
		return errors.New("message content is required")
	}

	document, err := UnmarshalDocument(raw.Content, *raw.Type)
	if err != nil {
		return err
	}

	m.Type = *raw.Type
	m.Content = document
	return nil
}
