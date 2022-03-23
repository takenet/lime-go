package lime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
)

type MessageSender interface {
	SendMessage(ctx context.Context, msg *Message) error
}

type MessageReceiver interface {
	ReceiveMessage(ctx context.Context) (*Message, error)
	MsgChan() <-chan *Message
}

type NotificationSender interface {
	SendNotification(ctx context.Context, not *Notification) error
}

type NotificationReceiver interface {
	ReceiveNotification(ctx context.Context) (*Notification, error)
	NotChan() <-chan *Notification
}

type RequestCommandSender interface {
	SendRequestCommand(ctx context.Context, cmd *RequestCommand) error
}

type ResponseCommandSender interface {
	SendResponseCommand(ctx context.Context, cmd *ResponseCommand) error
}

type RequestCommandReceiver interface {
	ReceiveRequestCommand(ctx context.Context) (*RequestCommand, error)
	ReqCmdChan() <-chan *RequestCommand
}

type ResponseCommandReceiver interface {
	ReceiveResponseCommand(ctx context.Context) (*ResponseCommand, error)
	RespCmdChan() <-chan *ResponseCommand
}

type CommandProcessor interface {
	ProcessCommand(ctx context.Context, cmd *RequestCommand) (*ResponseCommand, error)
}

type Receiver interface {
	MessageReceiver
	NotificationReceiver
	RequestCommandReceiver
	ResponseCommandReceiver
}

// Sender defines a service for sending envelopes to a remote party.
type Sender interface {
	MessageSender
	NotificationSender
	RequestCommandSender
	ResponseCommandSender
}

// ChannelModule defines a proxy interface for executing actions to the envelope channels.
type ChannelModule interface {
	StateChanged(ctx context.Context, state SessionState) // StateChanged is called when the session state is changed.
	Receiving(ctx context.Context, env envelope) envelope // Receiving is called when an envelope is being received by the channel.
	Sending(ctx context.Context, env envelope) envelope   // Sending is called when an envelope is being sent by the channel.
}

type channel struct {
	transport     Transport
	sessionID     string
	remoteNode    Node
	localNode     Node
	state         SessionState
	stateMu       sync.RWMutex
	inMsgChan     chan *Message
	inNotChan     chan *Notification
	inReqCmdChan  chan *RequestCommand
	inRespCmdChan chan *ResponseCommand
	inSesChan     chan *Session
	sendMu        sync.Mutex
	rcvMu         sync.Mutex
	startRcv      sync.Once
	stopRcv       sync.Once
	rcvDone       chan struct{}
	client        bool

	processingCmds   map[string]chan *ResponseCommand
	processingCmdsMu sync.RWMutex

	cancel context.CancelFunc // The function for cancelling the listener goroutine
}

func newChannel(t Transport, bufferSize int) *channel {
	if t == nil || reflect.ValueOf(t).IsNil() {
		panic("transport cannot be nil")
	}

	c := channel{
		transport:        t,
		state:            SessionStateNew,
		inMsgChan:        make(chan *Message, bufferSize),
		inNotChan:        make(chan *Notification, bufferSize),
		inReqCmdChan:     make(chan *RequestCommand, bufferSize),
		inRespCmdChan:    make(chan *ResponseCommand, bufferSize),
		inSesChan:        make(chan *Session, 1),
		rcvDone:          make(chan struct{}),
		processingCmds:   make(map[string]chan *ResponseCommand),
		processingCmdsMu: sync.RWMutex{},
	}
	return &c
}

func (c *channel) Established() bool {
	return c.State() == SessionStateEstablished && c.transport.Connected()
}

func (c *channel) startReceiver() {
	defer c.rcvMu.Unlock()
	c.rcvMu.Lock()

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go receiveFromTransport(ctx, c, c.rcvDone)
}

func (c *channel) stopReceiver() {
	defer c.rcvMu.Unlock()
	c.rcvMu.Lock()

	if c.cancel != nil {
		c.cancel()
		<-c.rcvDone
	}
}

func (c *channel) setState(state SessionState) {
	c.setStateWLock(state)

	switch state {
	case SessionStateEstablished:
		c.startRcv.Do(c.startReceiver)
	case SessionStateFinished, SessionStateFailed:
		c.stopRcv.Do(c.stopReceiver)
	}
}

func (c *channel) setStateWLock(state SessionState) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if state.Step() < c.state.Step() {
		panic(fmt.Errorf("cannot change from state %s to %s", c.state, state))
	}

	c.state = state
}

func (c *channel) MsgChan() <-chan *Message {
	return c.inMsgChan
}

func (c *channel) NotChan() <-chan *Notification {
	return c.inNotChan
}

func (c *channel) ReqCmdChan() <-chan *RequestCommand {
	return c.inReqCmdChan
}
func (c *channel) RespCmdChan() <-chan *ResponseCommand {
	return c.inRespCmdChan
}

func receiveFromTransport(ctx context.Context, c *channel, done chan<- struct{}) {
	defer func() {
		close(done)
		close(c.inMsgChan)
		close(c.inNotChan)
		close(c.inReqCmdChan)
		close(c.inRespCmdChan)
		close(c.inSesChan)
	}()

	for c.Established() {
		env, err := c.transport.Receive(ctx)
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("receiveFromTransport: %v", err)
			}
			return
		}

		switch e := env.(type) {
		case *Message:
			select {
			case <-ctx.Done():
				return
			case c.inMsgChan <- e:
			}
		case *Notification:
			select {
			case <-ctx.Done():
				return
			case c.inNotChan <- e:
			}
		case *RequestCommand:
			select {
			case <-ctx.Done():
				return
			case c.inReqCmdChan <- e:
			}
		case *ResponseCommand:
			if !c.trySubmitCommandResult(e) {
				select {
				case <-ctx.Done():
					return
				case c.inRespCmdChan <- e:
				}
			}
		case *Session:
			select {
			case <-ctx.Done():
				return
			case c.inSesChan <- e:
				// If a session is received while established,
				// the receiver goroutine can stop.
				if c.client {
					c.setStateWLock(e.State)
				}
				return
			}
		default:
			panic(fmt.Errorf("unknown envelope type %v", reflect.ValueOf(e)))
		}
	}
}

func (c *channel) ID() string {
	return c.sessionID
}

func (c *channel) RemoteNode() Node {
	return c.remoteNode
}

func (c *channel) LocalNode() Node {
	return c.localNode
}

func (c *channel) State() SessionState {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

func (c *channel) sendSession(ctx context.Context, ses *Session) error {
	if err := c.ensureTransportOK("send session"); err != nil {
		return err
	}
	// check the current channel state
	state := c.State()
	if state == SessionStateFinished || state == SessionStateFailed {
		return fmt.Errorf("send session: cannot do in the %v state", state)
	}

	err := c.transport.Send(ctx, ses)
	if err != nil {
		return fmt.Errorf("send session: transport error: %w", err)
	}
	return nil
}
func (c *channel) receiveSession(ctx context.Context) (*Session, error) {
	if ctx == nil {
		panic("nil context")
	}

	state := c.State()

	switch state {
	case SessionStateFinished:
		return nil, fmt.Errorf("receive session: cannot do in the %v state", state)
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

	if err := c.ensureTransportOK("receive session"); err != nil {
		return nil, err
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
	return c.sendToTransport(ctx, msg, "send message")
}

func (c *channel) ReceiveMessage(ctx context.Context) (*Message, error) {
	if err := c.ensureEstablished("receive message"); err != nil {
		return nil, err
	}

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
	return c.sendToTransport(ctx, not, "send notification")
}

func (c *channel) ReceiveNotification(ctx context.Context) (*Notification, error) {
	if err := c.ensureEstablished("receive notification"); err != nil {
		return nil, err
	}

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

func (c *channel) SendRequestCommand(ctx context.Context, cmd *RequestCommand) error {
	return c.sendToTransport(ctx, cmd, "send request command")
}

func (c *channel) ReceiveRequestCommand(ctx context.Context) (*RequestCommand, error) {
	if err := c.ensureEstablished("receive request command"); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive request command: %w", ctx.Err())
	case cmd, ok := <-c.inReqCmdChan:
		if !ok {
			return nil, errors.New("receive request command: channel closed")
		}
		return cmd, nil
	}
}

func (c *channel) SendResponseCommand(ctx context.Context, cmd *ResponseCommand) error {
	return c.sendToTransport(ctx, cmd, "send response command")
}

func (c *channel) ReceiveResponseCommand(ctx context.Context) (*ResponseCommand, error) {
	if err := c.ensureEstablished("receive response command"); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive response command: %w", ctx.Err())
	case cmd, ok := <-c.inRespCmdChan:
		if !ok {
			return nil, errors.New("receive response command: channel closed")
		}
		return cmd, nil
	}
}

func (c *channel) ProcessCommand(ctx context.Context, reqCmd *RequestCommand) (*ResponseCommand, error) {
	return c.processCommand(ctx, c, reqCmd)
}

func (c *channel) Close() error {
	c.stopRcv.Do(c.stopReceiver)
	if c.transport.Connected() {
		return c.transport.Close()
	}

	return nil
}

func (c *channel) sendToTransport(ctx context.Context, e envelope, action string) error {
	if e == nil || reflect.ValueOf(e).IsNil() {
		panic(fmt.Errorf("%v: envelope cannot be nil", action))
	}
	if err := c.ensureEstablished(action); err != nil {
		return err
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	if err := c.transport.Send(ctx, e); err != nil {
		return fmt.Errorf("%v: %w", action, err)
	}

	return nil
}

func (c *channel) ensureEstablished(action string) error {
	return c.ensureState(SessionStateEstablished, action)
}

func (c *channel) ensureState(state SessionState, action string) error {
	if err := c.ensureTransportOK(action); err != nil {
		return err
	}

	s := c.State()
	if s != state {
		return fmt.Errorf("%v: cannot do in the %v state", action, s)
	}
	return nil
}

func (c *channel) ensureTransportOK(action string) error {
	if c.transport == nil || reflect.ValueOf(c.transport).IsNil() {
		return fmt.Errorf("%v: transport is nil", action)
	}

	if !c.transport.Connected() {
		return fmt.Errorf("%v: transport is not connected", action)
	}
	return nil
}

func (c *channel) processCommand(ctx context.Context, sender RequestCommandSender, reqCmd *RequestCommand) (*ResponseCommand, error) {
	if reqCmd == nil {
		panic("process command: command cannot be nil")
	}
	if reqCmd.ID == "" {
		panic("process command: invalid command id")
	}

	c.processingCmdsMu.Lock()

	if _, ok := c.processingCmds[reqCmd.ID]; ok {
		c.processingCmdsMu.Unlock()
		return nil, errors.New("process command: the command id is already in use")
	}

	respChan := make(chan *ResponseCommand, 1)
	c.processingCmds[reqCmd.ID] = respChan
	c.processingCmdsMu.Unlock()

	defer func() {
		c.processingCmdsMu.Lock()
		delete(c.processingCmds, reqCmd.ID)
		c.processingCmdsMu.Unlock()
	}()

	err := sender.SendRequestCommand(ctx, reqCmd)
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

func (c *channel) trySubmitCommandResult(respCmd *ResponseCommand) bool {
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

// RcvDone signals when the channel receiver goroutine is done.
// This usually indicates that the session with the remote node was finished.
func (c *channel) RcvDone() <-chan struct{} {
	return c.rcvDone
}
