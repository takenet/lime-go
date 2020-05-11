package lime

import (
	"encoding/json"
	"errors"
)

// Provides the transport of a Content between nodes in a network.
type Message struct {
	Envelope
	// MIME declaration of the Content type of the message.
	Type MediaType `json:"type"`
	// Message body Content
	Content Document `json:"content"`
}

func (m *Message) SetContent(d Document) {
	m.Content = d
	m.Type = d.GetMediaType()
}

// Wrapper for custom marshalling
type MessageWrapper struct {
	EnvelopeWrapper
	Type    *MediaType       `json:"type"`
	Content *json.RawMessage `json:"content"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	mw, err := m.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(mw)
}

func (m *Message) UnmarshalJSON(b []byte) error {
	mw := MessageWrapper{}
	err := json.Unmarshal(b, &mw)
	if err != nil {
		return err
	}

	message := Message{}
	err = message.populate(&mw)
	if err != nil {
		return err
	}

	*m = message
	return nil
}

func (m *Message) toWrapper() (MessageWrapper, error) {
	ew, err := m.Envelope.toWrapper()
	if err != nil {
		return MessageWrapper{}, err
	}

	if m.Content == nil {
		return MessageWrapper{}, errors.New("message content is required")
	}
	b, err := json.Marshal(m.Content)
	if err != nil {
		return MessageWrapper{}, err
	}
	r := json.RawMessage(b)

	return MessageWrapper{
		EnvelopeWrapper: ew,
		Type:            &m.Type,
		Content:         &r,
	}, nil
}

func (m *Message) populate(mw *MessageWrapper) error {
	err := m.Envelope.populate(&mw.EnvelopeWrapper)
	if err != nil {
		return err
	}

	// Create the document type instance and unmarshal the json To it
	if mw.Type == nil {
		return errors.New("message type is required")
	}

	if mw.Content == nil {
		return errors.New("message content is required")
	}

	factory, err := GetDocumentFactory(*mw.Type)
	if err != nil {
		return err
	}

	document := factory()
	err = json.Unmarshal(*mw.Content, &document)
	if err != nil {
		return err
	}

	m.Type = *mw.Type
	m.Content = document
	return nil
}
