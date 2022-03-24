package lime

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Notification provides information about events associated to a Message.
// It can be originated by an intermediate node, like a server, or by the destination of the message.
type Notification struct {
	Envelope
	// Event Related event to the notification
	Event NotificationEvent
	// In the case of a failed event, the Reason value brings more details about the problem.
	Reason *Reason
}

func (not *Notification) SetEvent(event NotificationEvent) *Notification {
	not.Event = event
	return not
}

func (not *Notification) SetFailed(reason *Reason) *Notification {
	not.Event = NotificationEventFailed
	not.Reason = reason
	return not
}

func (not Notification) MarshalJSON() ([]byte, error) {
	raw, err := not.toRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (not *Notification) UnmarshalJSON(b []byte) error {
	raw := rawEnvelope{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	notification := Notification{}
	err = notification.populate(&raw)
	if err != nil {
		return err
	}

	*not = notification
	return nil
}

func (not *Notification) toRawEnvelope() (*rawEnvelope, error) {
	raw, err := not.Envelope.toRawEnvelope()
	if err != nil {
		return nil, err
	}

	if not.Event != "" {
		raw.Event = &not.Event
	}

	raw.Reason = not.Reason

	return raw, nil
}

func (not *Notification) populate(raw *rawEnvelope) error {
	err := not.Envelope.populate(raw)
	if err != nil {
		return err
	}

	if raw.Event == nil {
		return errors.New("notification event is required")
	}

	not.Event = *raw.Event
	not.Reason = raw.Reason

	return nil
}

// NotificationEvent represent the events that can happen in the message pipeline.
type NotificationEvent string

const (
	// NotificationEventAccepted The message was received and accepted by the server.
	// This event is similar To 'received' but is emitted by an intermediate node (hop) and not by the message's final destination.
	NotificationEventAccepted = NotificationEvent("accepted")
	// NotificationEventDispatched The message was dispatched to the destination by the server.
	// This event is similar To the 'consumed' but is emitted by an intermediate node (hop) and not by the message's final destination.
	NotificationEventDispatched = NotificationEvent("dispatched")
	// NotificationEventReceived The node has received the message.
	NotificationEventReceived = NotificationEvent("received")
	// NotificationEventConsumed The node has consumed the Content of the message.
	NotificationEventConsumed = NotificationEvent("consumed")
	// NotificationEventFailed A problem occurred during the processing of the message.
	// In this case, the reason property of the notification should be present.
	NotificationEventFailed = NotificationEvent("failed")
)

func (e *NotificationEvent) Validate() error {
	switch *e {
	case NotificationEventAccepted, NotificationEventDispatched, NotificationEventReceived, NotificationEventConsumed, NotificationEventFailed:
		return nil
	}

	return fmt.Errorf("invalid notification event '%v'", e)
}

func (e NotificationEvent) MarshalText() ([]byte, error) {
	err := e.Validate()
	if err != nil {
		return []byte{}, err
	}
	return []byte(e), nil
}

func (e *NotificationEvent) UnmarshalText(text []byte) error {
	event := NotificationEvent(text)
	err := event.Validate()
	if err != nil {
		return err
	}
	*e = event
	return nil
}
