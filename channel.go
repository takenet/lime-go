package lime

import (
	"context"
	"errors"
	"fmt"
)

type MessageSender interface {
	SendMessage(m Message) error
}

type MessageReceiver interface {
	ReceiveMessage() (Message, error)
}

type NotificationSender interface {
	SendNotification(n Notification) error
}

type NotificationReceiver interface {
	ReceiveNotification() (Notification, error)
}

type CommandSender interface {
	SendCommand(c Command) error
}

type CommandReceiver interface {
	ReceiveCommand() (Command, error)
}

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, c Command) (Command, error)
}

type SessionSender interface {
	SendSession(s Session) error
}

type SessionReceiver interface {
	ReceiveSession() (Session, error)
}

// Base type for the protocol communication channels.
type Channel struct {
	// The current session transport.
	Transport Transport

	// The session ID.
	SessionID string

	// The remote party identifier.
	RemoteNode Node

	// The local node identifier.
	LocalNode Node

	// The current session state.
	State SessionState

	sendChan                chan Envelope
	receiveMessageChan      chan Message
	receiveNotificationChan chan Notification
	receiveCommandChan      chan Command
	receiveSessionChan      chan Session
}

func (c *Channel) SendSession(s Session) error {
	// check the current channel state
	if c.State == SessionStateFinished || c.State == SessionStateFailed {
		return fmt.Errorf("cannot send a session in the %v state", c.State)
	}

	if s.State == SessionStateFinishing || s.State == SessionStateFinished || s.State == SessionStateFailed {
		// TODO: send a signal to stop the listener goroutine
	}

	return c.Transport.Send(&s)
}
func (c *Channel) ReceiveSession() (Session, error) {
	switch c.State {
	case SessionStateFinished:
		return Session{}, fmt.Errorf("cannot receive a session in the %v session state", c.State)
	case SessionStateEstablished:
		s, ok := <-c.receiveSessionChan
		if !ok {
			return Session{}, errors.New("receiver channel is closed")
		}
		return s, nil
	}

	e, err := c.Transport.Receive()
	if err != nil {
		return Session{}, err
	}

	s, ok := e.(*Session)
	if !ok {
		return Session{}, errors.New("an unexpected envelope was received from the transport")
	}

	return *s, nil
}

type ClientChannel struct {
	Channel
}
