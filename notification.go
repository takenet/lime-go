package lime

import (
	"encoding/json"
	"fmt"
)

// Notification Information about events associated to a Message in a Session.
// Can be originated by a server or by the Message destination Node.
type Notification struct {
	EnvelopeBase

	// Event Related event To the notification
	Event NotificationEvent

	// Reason In the case of a failed event, brings more details about the problem.
	Reason Reason
}

// NotificationWrapper Wrapper for custom marshalling
type NotificationWrapper struct {
	EnvelopeBaseWrapper
	Event  NotificationEvent `json:"event,omitempty"`
	Reason *Reason           `json:"reason,omitempty"`
}

func (n Notification) MarshalJSON() ([]byte, error) {
	nw, err := n.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(nw)
}

func (n *Notification) UnmarshalJSON(b []byte) error {
	nw := NotificationWrapper{}
	err := json.Unmarshal(b, &nw)
	if err != nil {
		return err
	}

	command := Notification{}
	err = command.populate(&nw)
	if err != nil {
		return err
	}

	*n = command
	return nil
}

func (n *Notification) toWrapper() (NotificationWrapper, error) {
	ew, err := n.EnvelopeBase.toWrapper()
	if err != nil {
		return NotificationWrapper{}, err
	}

	nw := NotificationWrapper{
		EnvelopeBaseWrapper: ew,
		Event:               n.Event,
	}

	if n.Reason != (Reason{}) {
		nw.Reason = &n.Reason
	}

	return nw, nil
}

func (n *Notification) populate(nw *NotificationWrapper) error {
	err := n.EnvelopeBase.populate(&nw.EnvelopeBaseWrapper)
	if err != nil {
		return err
	}

	n.Event = nw.Event

	if nw.Reason != nil {
		n.Reason = *nw.Reason
	}

	return nil
}

// NotificationEvent Events that can happen in the message pipeline.
type NotificationEvent string

const (
	// NotificationEventAccepted The message was received and accepted by the server.
	// This event is similar To 'received' but is emitted by an intermediate node (hop) and not by the message's final destination.
	NotificationEventAccepted = NotificationEvent("accepted")
	// NotificationEventDispatched The message was dispatched To the destination by the server.
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
