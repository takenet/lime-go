package lime

import (
	"context"
	"errors"
	"fmt"
)

type MessageSender interface {
	SendMessage(m *Message) error
}

type MessageReceiver interface {
	ReceiveMessage() (*Message, error)
}

type NotificationSender interface {
	SendNotification(n *Notification) error
}

type NotificationReceiver interface {
	ReceiveNotification() (*Notification, error)
}

type CommandSender interface {
	SendCommand(c *Command) error
}

type CommandReceiver interface {
	ReceiveCommand() (*Command, error)
}

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, c *Command) (*Command, error)
}

type SessionSender interface {
	SendSession(s *Session) error
}

type SessionReceiver interface {
	ReceiveSession() (*Session, error)
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

	outBuf            chan Envelope
	inMessageBuf      chan Envelope
	inNotificationBuf chan Envelope
	inCommandBuf      chan Envelope
	inSessionBuf      chan Envelope
}

func (c *Channel) SendSession(s *Session) error {
	// check the current channel state
	if c.State == SessionStateFinished || c.State == SessionStateFailed {
		return fmt.Errorf("cannot send a session in the %v state", c.State)
	}

	if s.State == SessionStateFinishing || s.State == SessionStateFinished || s.State == SessionStateFailed {
		// TODO: signal to stop the listener goroutine
	}

	return c.Transport.Send(s)
}
func (c *Channel) ReceiveSession() (*Session, error) {
	if err := c.ensureTransportOK(); err != nil {
		return nil, err
	}

	switch c.State {
	case SessionStateFinished:
		return nil, fmt.Errorf("cannot receive a session in the %v session state", c.State)
	case SessionStateEstablished:
		e, ok := <-c.inSessionBuf
		if !ok {
			return nil, errors.New("receiver channel is closed")
		}
		s, ok := e.(*Session)
		if !ok {
			panic("unexpected envelope type was received from session buffer")
		}

		return s, nil
	}

	e, err := c.Transport.Receive()
	if err != nil {
		return nil, err
	}

	s, ok := e.(*Session)
	if !ok {
		return nil, errors.New("an unexpected envelope was received from the transport")
	}

	return s, nil
}

func (c *Channel) SendMessage(m *Message) error {
	return c.sendToBuffer(m)
}

func (c *Channel) ReceiveMessage() (*Message, error) {
	e, err := c.receiveFromBuffer(c.inMessageBuf)
	if err != nil {
		return nil, err
	}

	m, ok := e.(*Message)
	if !ok {
		panic(fmt.Errorf("unexpected envelope type received from buffer. expected *Message, received %T", e))
	}

	return m, nil
}

func (c *Channel) sendToBuffer(e Envelope) error {
	if err := c.ensureEstablished("send"); err != nil {
		return err
	}

	if err := c.ensureTransportOK(); err != nil {
		return err
	}

	if c.outBuf == nil {
		return errors.New("outBuf is not defined")
	}

	c.outBuf <- e
	return nil
}
func (c *Channel) receiveFromBuffer(buf chan Envelope) (Envelope, error) {
	return nil, nil
}

func (c *Channel) ensureEstablished(action string) error {
	if c.State != SessionStateEstablished {
		return fmt.Errorf("cannot %v in the %v state", action, c.State)
	}
	return nil
}

func (c *Channel) ensureTransportOK() error {
	if c.Transport == nil {
		return errors.New("transport is not defined")
	}

	if !c.Transport.IsConnected() {
		return errors.New("transport is not connected")
	}
	return nil
}

type ClientChannel struct {
	Channel
}
