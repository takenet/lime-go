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
	t := d.GetMediaType()
	m.Type = t
}

// Wrapper for custom marshalling
type MessageWrapper struct {
	EnvelopeWrapper
	Type    *MediaType      `json:"type"`
	Content json.RawMessage `json:"content"`
}

func (m Message) MarshalJSON() ([]byte, error) {
	mw, err := m.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(mw)
}

func (m *Message) UnmarshalJSON(b []byte) error {
	mj := MessageWrapper{}
	err := json.Unmarshal(b, &mj)
	if err != nil {
		return err
	}

	message := Message{}
	err = message.Envelope.populate(&mj.EnvelopeWrapper)
	if err != nil {
		return err
	}

	err = message.populate(&mj)
	if err != nil {
		return err
	}

	*m = message
	return nil
}

func (m *Message) toWrapper() (MessageWrapper, error) {
	if m.Content == nil {
		return MessageWrapper{}, errors.New("message Content is required")
	}

	b, err := json.Marshal(m.Content)
	if err != nil {
		return MessageWrapper{}, err
	}
	r := json.RawMessage(b)

	ew, err := m.Envelope.toWrapper()
	if err != nil {
		return MessageWrapper{}, err
	}

	return MessageWrapper{
		EnvelopeWrapper: ew,
		Type:            &m.Type,
		Content:         r,
	}, nil
}

func (m *Message) populate(mj *MessageWrapper) error {
	// Create the document type instance and unmarshal the json To it
	if mj.Type == nil {
		return errors.New("type is required")
	}

	factory, err := GetDocumentFactory(*mj.Type)
	if err != nil {
		return err
	}

	document := factory()
	err = json.Unmarshal(mj.Content, &document)
	if err != nil {
		return err
	}

	m.Type = *mj.Type
	m.Content = document
	return nil
}
