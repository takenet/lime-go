package lime

import "fmt"

// Transport information about events associated To a message in a session.
// Can be originated by a server or by the message destination node.
type Notification struct {
	Envelope
	// Related event To the notification
	Event NotificationEvent `json:"event,omitempty"`
	// In the case of a failed event, brings more details about the problem.
	Reason *Reason `json:"reason,omitempty"`
}

// Events that can happen in the message pipeline.
type NotificationEvent string

const (
	// The message was received and accepted by the server.
	// This event is similar To 'received' but is emitted by an intermediate node (hop) and not by the message's final destination.
	NotificationEventAccepted = NotificationEvent("accepted")
	// The message was dispatched To the destination by the server.
	// This event is similar To the 'consumed' but is emitted by an intermediate node (hop) and not by the message's final destination.
	NotificationEventDispatched = NotificationEvent("dispatched")
	// The node has received the message.
	NotificationEventReceived = NotificationEvent("received")
	// The node has consumed the Content of the message.
	NotificationEventConsumed = NotificationEvent("consumed")
	// A problem occurred during the processing of the message.
	// In this case, the reason property of the notification should be present.
	NotificationEventFailed = NotificationEvent("failed")
)

func (e *NotificationEvent) IsValid() error {
	switch *e {
	case NotificationEventAccepted, NotificationEventDispatched, NotificationEventReceived, NotificationEventConsumed, NotificationEventFailed:
		return nil
	}

	return fmt.Errorf("invalid notification event '%v'", e)
}

func (e NotificationEvent) MarshalText() ([]byte, error) {
	err := e.IsValid()
	if err != nil {
		return []byte{}, err
	}
	return []byte(e), nil
}

func (e *NotificationEvent) UnmarshalText(text []byte) error {
	event := NotificationEvent(text)
	err := event.IsValid()
	if err != nil {
		return err
	}
	*e = event
	return nil
}
