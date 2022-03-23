package lime

import (
	"encoding/json"
	"errors"
)

// Message encapsulates a document for transport between nodes in a network.
type Message struct {
	Envelope
	// MIME Type declaration for the Content of the message.
	Type MediaType `json:"type"`
	// Content represents the Message body content
	Content Document `json:"content"`
}

func (msg *Message) SetContent(d Document) *Message {
	msg.Content = d
	msg.Type = d.MediaType()
	return msg
}

func (msg *Message) MarshalJSON() ([]byte, error) {
	raw, err := msg.toRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (msg *Message) UnmarshalJSON(b []byte) error {
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

	*msg = message
	return nil
}

func (msg *Message) toRawEnvelope() (*rawEnvelope, error) {
	raw, err := msg.Envelope.toRawEnvelope()
	if err != nil {
		return nil, err
	}

	if msg.Content == nil {
		return nil, errors.New("message content is required")
	}
	b, err := json.Marshal(msg.Content)
	if err != nil {
		return nil, err
	}
	content := json.RawMessage(b)

	raw.Type = &msg.Type
	raw.Content = &content

	return raw, nil
}

func (msg *Message) populate(raw *rawEnvelope) error {
	err := msg.Envelope.populate(raw)
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

	msg.Type = *raw.Type
	msg.Content = document
	return nil
}

// Notification creates a notification for the current message.
func (msg *Message) Notification(event NotificationEvent) *Notification {
	return &Notification{
		Envelope: Envelope{
			ID:   msg.ID,
			From: msg.To,
			To:   msg.Sender(),
		},
		Event: event,
	}
}

// FailedNotification creates a notification for the current message with
// the 'failed' event.
func (msg *Message) FailedNotification(reason *Reason) *Notification {
	not := msg.Notification(NotificationEventFailed)
	not.Reason = reason
	return not
}
