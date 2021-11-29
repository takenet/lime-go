package lime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

type MessageSender interface {
	SendMessage(ctx context.Context, msg *Message) error
}

type MessageReceiver interface {
	ReceiveMessage(ctx context.Context) (*Message, error)
}

type NotificationSender interface {
	SendNotification(ctx context.Context, not *Notification) error
}

type NotificationReceiver interface {
	ReceiveNotification(ctx context.Context) (*Notification, error)
}

type CommandSender interface {
	SendCommand(ctx context.Context, cmd *Command) error
}

type CommandReceiver interface {
	ReceiveCommand(ctx context.Context) (*Command, error)
}

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, cmd *Command) (*Command, error)
}

type SessionInfoProvider interface {
	// GetID gets the session ID.
	GetID() string

	// GetRemoteNode gets the remote party identifier.
	GetRemoteNode() Node

	// GetLocalNode gets the local node identifier.
	GetLocalNode() Node

	// GetState gets the current session state.
	GetState() SessionState
}

// ChannelModule defines a proxy interface for executing actions to the envelope channels.
type ChannelModule interface {
	// OnStateChanged is called when the session state is changed.
	OnStateChanged(ctx context.Context, state SessionState)

	// OnReceiving is called when an envelope is being received by the channel.
	OnReceiving(ctx context.Context, env Envelope)

	// OnSending is called when an envelope is being sent by the channel.
	OnSending(ctx context.Context, env Envelope)
}

type channel struct {
	transport  Transport
	sessionID  string
	remoteNode Node
	localNode  Node
	state      SessionState
	outChan    chan Envelope
	inMsgChan  chan *Message
	inNotChan  chan *Notification
	inCmdChan  chan *Command
	inSesChan  chan *Session
	errChan    chan error

	processingCmds   map[string]chan *Command
	processingCmdsMu sync.RWMutex

	cancel context.CancelFunc // The function for cancelling the send/receive goroutines
}

func newChannel(t Transport, bufferSize int) (*channel, error) {
	if t == nil || reflect.ValueOf(t).IsNil() {
		return nil, errors.New("transport cannot be nil")
	}

	c := channel{
		transport:        t,
		state:            SessionStateNew,
		outChan:          make(chan Envelope, bufferSize),
		inMsgChan:        make(chan *Message, bufferSize),
		inNotChan:        make(chan *Notification, bufferSize),
		inCmdChan:        make(chan *Command, bufferSize),
		inSesChan:        make(chan *Session, bufferSize),
		errChan:          make(chan error, 2),
		processingCmds:   make(map[string]chan *Command),
		processingCmdsMu: sync.RWMutex{},
	}
	return &c, nil
}

func (c *channel) Established() bool {
	return c.state == SessionStateEstablished && c.transport.IsConnected()
}

func (c *channel) startGoroutines() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go receiveFromTransport(ctx, c)
	go sendToTransport(ctx, c)
}

func (c *channel) setState(state SessionState) error {
	if state.Step() < c.state.Step() {
		return fmt.Errorf("cannot change from state %s to %s", c.state, state)
	}

	c.state = state

	switch state {
	case SessionStateEstablished:
		c.startGoroutines()
	case SessionStateFinished, SessionStateFailed:
		if c.cancel != nil {
			c.cancel()
		}
	}
	return nil
}

func (c *channel) MsgChan() <-chan *Message {
	return c.inMsgChan
}

func (c *channel) NotChan() <-chan *Notification {
	return c.inNotChan
}

func (c *channel) CmdChan() <-chan *Command {
	return c.inCmdChan
}

func (c *channel) SesChan() <-chan *Session {
	return c.inSesChan
}

func (c *channel) ErrChan() <-chan error {
	return c.errChan
}

func receiveFromTransport(ctx context.Context, c *channel) {
	for c.Established() {
		env, err := c.transport.Receive(ctx)
		if err != nil {
			c.errChan <- err
			break
		}

		switch e := env.(type) {
		case *Message:
			c.inMsgChan <- e
		case *Notification:
			c.inNotChan <- e
		case *Command:
			if !c.trySubmitCommandResult(e) {
				c.inCmdChan <- e
			}
		case *Session:
			c.inSesChan <- e
		default:
			panic(fmt.Errorf("unknown envelope type %v", e))
		}
	}
	close(c.inMsgChan)
	close(c.inNotChan)
	close(c.inCmdChan)
	close(c.inSesChan)
}

func sendToTransport(ctx context.Context, c *channel) {
	for c.Established() {
		select {
		case <-ctx.Done():
			break
		case e, ok := <-c.outChan:
			if !ok {
				return
			}
			err := c.transport.Send(ctx, e)
			if err != nil {
				c.errChan <- err
				break
			}
		}
	}
	close(c.outChan)
}

func (c *channel) GetID() string {
	return c.sessionID
}

func (c *channel) GetRemoteNode() Node {
	return c.remoteNode
}

func (c *channel) GetLocalNode() Node {
	return c.localNode
}

func (c *channel) GetState() SessionState {
	return c.state
}

func (c *channel) sendSession(ctx context.Context, ses *Session) error {
	if err := c.ensureTransportOK("send session"); err != nil {
		return err
	}
	// check the current channel state
	if c.state == SessionStateFinished || c.state == SessionStateFailed {
		return fmt.Errorf("send session: cannot do in the %v state", c.state)
	}

	err := c.transport.Send(ctx, ses)
	if err != nil {
		return fmt.Errorf("send session: transport error: %w", err)
	}
	return nil
}
func (c *channel) receiveSession(ctx context.Context) (*Session, error) {
	if err := c.ensureTransportOK("receive session"); err != nil {
		return nil, err
	}

	switch c.state {
	case SessionStateFinished:
		return nil, fmt.Errorf("receive session: cannot do in the %v state", c.state)
	case SessionStateEstablished:
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("receive session: %w", ctx.Err())
		case s, ok := <-c.inSesChan:
			if !ok {
				return nil, errors.New("receive session: channel closed")
			}
			return s, nil
		}
	}

	env, err := c.transport.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("receive session: transport error: %w", err)
	}

	ses, ok := env.(*Session)
	if !ok {
		return nil, errors.New("receive session: unexpected envelope type")
	}

	return ses, nil
}

func (c *channel) SendMessage(ctx context.Context, msg *Message) error {
	return c.sendToBuffer(ctx, msg, "send message")
}

func (c *channel) ReceiveMessage(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive message: %w", ctx.Err())
	case msg, ok := <-c.inMsgChan:
		if !ok {
			return nil, errors.New("receive message: channel closed")
		}
		return msg, nil
	}
}
func (c *channel) SendNotification(ctx context.Context, not *Notification) error {
	return c.sendToBuffer(ctx, not, "send notification")
}

func (c *channel) ReceiveNotification(ctx context.Context) (*Notification, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive notification: %w", ctx.Err())
	case not, ok := <-c.inNotChan:
		if !ok {
			return nil, errors.New("receive notification: channel closed")
		}
		return not, nil
	}
}

func (c *channel) SendCommand(ctx context.Context, cmd *Command) error {
	return c.sendToBuffer(ctx, cmd, "send command")
}

func (c *channel) ReceiveCommand(ctx context.Context) (*Command, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive command: %w", ctx.Err())
	case cmd, ok := <-c.inCmdChan:
		if !ok {
			return nil, errors.New("receive command: channel closed")
		}
		return cmd, nil
	}
}

func (c *channel) ProcessCommand(ctx context.Context, reqCmd *Command) (*Command, error) {
	return c.processCommand(ctx, c, reqCmd)
}

func (c *channel) sendToBuffer(ctx context.Context, e Envelope, action string) error {
	if e == nil || reflect.ValueOf(e).IsNil() {
		return fmt.Errorf("%v: envelope cannot be nil", action)
	}
	if err := c.ensureEstablished(action); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("%v: %w", action, ctx.Err())
	case c.outChan <- e:
		return nil
	}
}

func (c *channel) ensureEstablished(action string) error {
	return c.ensureState(SessionStateEstablished, action)
}

func (c *channel) ensureState(state SessionState, action string) error {
	if err := c.ensureTransportOK(action); err != nil {
		return err
	}

	if c.state != state {
		return fmt.Errorf("%v: cannot do in the %v state", action, c.state)
	}
	return nil
}

func (c *channel) ensureTransportOK(action string) error {
	if c.transport == nil || reflect.ValueOf(c.transport).IsNil() {
		return fmt.Errorf("%v: transport is nil", action)
	}

	if !c.transport.IsConnected() {
		return fmt.Errorf("%v: transport is not connected", action)
	}
	return nil
}

func (c *channel) processCommand(ctx context.Context, sender CommandSender, reqCmd *Command) (*Command, error) {
	if reqCmd == nil {
		return nil, errors.New("process command: command cannot be nil")
	}
	if reqCmd.Status != "" {
		return nil, errors.New("process command: invalid command status")
	}
	if reqCmd.ID == "" {
		return nil, errors.New("process command: invalid command id")
	}

	c.processingCmdsMu.Lock()

	if _, ok := c.processingCmds[reqCmd.ID]; ok {
		c.processingCmdsMu.Unlock()
		return nil, errors.New("process command: the command id is already in use")
	}

	respChan := make(chan *Command, 1)
	c.processingCmds[reqCmd.ID] = respChan
	c.processingCmdsMu.Unlock()

	defer func() {
		c.processingCmdsMu.Lock()
		delete(c.processingCmds, reqCmd.ID)
		c.processingCmdsMu.Unlock()
	}()

	err := sender.SendCommand(ctx, reqCmd)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("process command: %w", ctx.Err())
	case respCmd := <-respChan:
		return respCmd, nil
	}
}

func (c *channel) trySubmitCommandResult(respCmd *Command) bool {
	if respCmd == nil {
		return false
	}

	c.processingCmdsMu.RLock()
	respChan, ok := c.processingCmds[respCmd.ID]
	c.processingCmdsMu.RUnlock()

	if !ok {
		return false
	}

	c.processingCmdsMu.Lock()
	delete(c.processingCmds, respCmd.ID)
	c.processingCmdsMu.Unlock()

	respChan <- respCmd
	return true
}
