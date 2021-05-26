package lime

import (
	"context"
	"errors"
	"fmt"
)

type MessageSender interface {
	SendMessage(ctx context.Context, m *Message) error
}

type MessageReceiver interface {
	ReceiveMessage(ctx context.Context) (*Message, error)
}

type NotificationSender interface {
	SendNotification(ctx context.Context, n *Notification) error
}

type NotificationReceiver interface {
	ReceiveNotification(ctx context.Context) (*Notification, error)
}

type CommandSender interface {
	SendCommand(ctx context.Context, c *Command) error
}

type CommandReceiver interface {
	ReceiveCommand(ctx context.Context) (*Command, error)
}

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, c *Command) (*Command, error)
}

type SessionSender interface {
	SendSession(ctx context.Context, s *Session) error
}

type SessionReceiver interface {
	ReceiveSession(ctx context.Context) (*Session, error)
}

type SessionInformation interface {
	// Get the session ID.
	getID() string

	// Get the remote party identifier.
	getRemoteNode() Node

	// Get the local node identifier.
	getLocalNode() Node

	// Get the current session state.
	getState() SessionState
}

// Channel Base type for the protocol communication channels.
type Channel struct {
	transport          Transport
	sessionID          string
	remoteNode         Node
	localNode          Node
	state              SessionState
	outChan            chan Envelope
	inMessageChan      chan Envelope
	inNotificationChan chan Envelope
	inCommandChan      chan Envelope
	inSessionChan      chan Envelope
}

func (c *Channel) SendSession(ctx context.Context, s *Session) error {
	// check the current channel state
	if c.state == SessionStateFinished || c.state == SessionStateFailed {
		return fmt.Errorf("cannot send a session in the %v state", c.state)
	}

	if s.State == SessionStateFinishing || s.State == SessionStateFinished || s.State == SessionStateFailed {
		// TODO: signal to stop the listener goroutine
	}

	return c.transport.Send(ctx, s)
}
func (c *Channel) ReceiveSession(ctx context.Context) (*Session, error) {
	if err := c.ensureTransportOK(); err != nil {
		return nil, err
	}

	switch c.state {
	case SessionStateFinished:
		return nil, fmt.Errorf("cannot receive a session in the %v session state", c.state)
	case SessionStateEstablished:
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case e, ok := <-c.inSessionChan:
			if !ok {
				return nil, errors.New("receiver channel is closed")
			}
			s, ok := e.(*Session)
			if !ok {
				panic("unexpected envelope type was received from session buffer")
			}
			return s, nil
		}
	}

	e, err := c.transport.Receive(ctx)
	if err != nil {
		return nil, err
	}

	s, ok := e.(*Session)
	if !ok {
		return nil, errors.New("an unexpected envelope was received from the transport")
	}

	return s, nil
}

func (c *Channel) SendMessage(ctx context.Context, m *Message) error {
	return c.sendToBuffer(ctx, m)
}

func (c *Channel) ReceiveMessage(ctx context.Context) (*Message, error) {
	e, err := c.receiveFromBuffer(ctx, c.inMessageChan)
	if err != nil {
		return nil, err
	}

	m, ok := e.(*Message)
	if !ok {
		panic(fmt.Errorf("unexpected envelope type received from buffer. expected *Message, received %T", e))
	}

	return m, nil
}

func (c *Channel) sendToBuffer(ctx context.Context, e Envelope) error {
	if err := c.ensureEstablished("send"); err != nil {
		return err
	}

	if err := c.ensureTransportOK(); err != nil {
		return err
	}

	if c.outChan == nil {
		return errors.New("outChan is not defined")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.outChan <- e:
		return nil
	}
}
func (c *Channel) receiveFromBuffer(ctx context.Context, buf chan Envelope) (Envelope, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case e := <-buf:
		return e, nil
	}
}

func (c *Channel) ensureEstablished(action string) error {
	if c.state != SessionStateEstablished {
		return fmt.Errorf("cannot %v in the %v state", action, c.state)
	}
	return nil
}

func (c *Channel) ensureTransportOK() error {
	if c.transport == nil {
		return errors.New("transport is not defined")
	}

	if !c.transport.IsConnected() {
		return errors.New("transport is not connected")
	}
	return nil
}

type ClientChannel struct {
	Channel
}

type ServerChannel struct {
	Channel
}
