package lime

import (
	"context"
	"errors"
	"fmt"
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

type SessionSender interface {
	SendSession(ctx context.Context, ses *Session) error
}

type SessionReceiver interface {
	ReceiveSession(ctx context.Context) (*Session, error)
}

type SessionInformation interface {
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
	commandProcessorChannel

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

	cancel context.CancelFunc // The function for cancelling the send/receive goroutines
}

func newChannel(t Transport, bufferSize int) (*channel, error) {
	if t == nil {
		return nil, errors.New("transport cannot be nil")
	}

	c := channel{
		transport: t,
		state:     SessionStateNew,
		outChan:   make(chan Envelope, bufferSize),
		inMsgChan: make(chan *Message, bufferSize),
		inNotChan: make(chan *Notification, bufferSize),
		inCmdChan: make(chan *Command, bufferSize),
		inSesChan: make(chan *Session, bufferSize),
	}
	return &c, nil
}

func (c *channel) startGoroutines() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	c.cancel = cancelFunc

	go receiveFromTransport(ctx, c)

}

func (c *channel) isEstablished() bool {
	return c.state == SessionStateEstablished && c.transport.IsConnected()
}

func receiveFromTransport(ctx context.Context, c *channel) {
	for c.isEstablished() {
		env, err := c.transport.Receive(ctx)
		if err != nil {
			// TODO: Handle error
			break
		}

		switch e := env.(type) {
		case *Message:
			c.inMsgChan <- e
		case *Notification:
			c.inNotChan <- e
		case *Command:
			if !c.commandProcessorChannel.TrySubmitCommandResult(e) {
				c.inCmdChan <- e
			}
		case *Session:
			c.inSesChan <- e
		default:
			// TODO: Handle unknown envelope type
			break
		}

	}
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

func (c *channel) SendSession(ctx context.Context, ses *Session) error {
	// check the current channel state
	if c.state == SessionStateFinished || c.state == SessionStateFailed {
		return fmt.Errorf("cannot send a session in the %v state", c.state)
	}

	if ses.State == SessionStateFinishing || ses.State == SessionStateFinished || ses.State == SessionStateFailed {
		// TODO: signal to stop the listener goroutine
	}

	return c.transport.Send(ctx, ses)
}
func (c *channel) receiveSession(ctx context.Context) (*Session, error) {
	if err := c.ensureTransportOK("receive session"); err != nil {
		return nil, err
	}

	switch c.state {
	case SessionStateFinished:
		return nil, fmt.Errorf("cannot receive a session in the %v session state", c.state)
	case SessionStateEstablished:
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case s, ok := <-c.inSesChan:
			if !ok {
				return nil, errors.New("session receiver channel is closed")
			}
			return s, nil
		}
	}

	env, err := c.transport.Receive(ctx)
	if err != nil {
		return nil, err
	}

	ses, ok := env.(*Session)
	if !ok {
		return nil, errors.New("an unexpected envelope was received from the transport")
	}

	return ses, nil
}

func (c *channel) SendMessage(ctx context.Context, msg *Message) error {
	return c.sendToBuffer(ctx, msg)
}

func (c *channel) ReceiveMessage(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-c.inMsgChan:
		if !ok {
			return nil, errors.New("message receiver channel is closed")
		}
		return msg, nil
	}
}
func (c *channel) SendNotification(ctx context.Context, not *Notification) error {
	return c.sendToBuffer(ctx, not)
}

func (c *channel) ReceiveNotification(ctx context.Context) (*Notification, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case not, ok := <-c.inNotChan:
		if !ok {
			return nil, errors.New("notification receiver channel is closed")
		}
		return not, nil
	}
}

func (c *channel) SendCommand(ctx context.Context, cmd *Command) error {
	return c.sendToBuffer(ctx, cmd)
}

func (c *channel) ReceiveCommand(ctx context.Context) (*Command, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case cmd, ok := <-c.inCmdChan:
		if !ok {
			return nil, errors.New("notification receiver channel is closed")
		}
		return cmd, nil
	}
}

func (c *channel) ProcessCommand(ctx context.Context, reqCmd *Command) (*Command, error) {
	return c.commandProcessorChannel.ProcessCommand(ctx, c, reqCmd)
}

func (c *channel) sendToBuffer(ctx context.Context, env Envelope) error {
	if env == nil {
		return errors.New("envelope cannot be nil")
	}

	if err := c.ensureEstablished("send"); err != nil {
		return err
	}

	if err := c.ensureTransportOK("send"); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.outChan <- env:
		return nil
	}
}

func (c *channel) ensureEstablished(action string) error {
	return c.ensureState(SessionStateEstablished, action)
}

func (c *channel) ensureState(state SessionState, action string) error {
	if c.state != state {
		return fmt.Errorf("cannot %v in the %v state", action, c.state)
	}
	return nil
}

func (c *channel) ensureTransportOK(action string) error {
	if c.transport == nil {
		return fmt.Errorf("cannot %v: transport is not defined", action)
	}

	if !c.transport.IsConnected() {
		return fmt.Errorf("cannot %v: transport is not connected", action)
	}
	return nil
}

type commandProcessorChannel struct {
	pendingCommands      map[string]chan *Command
	pendingCommandsMutex sync.RWMutex
}

func (c *commandProcessorChannel) ProcessCommand(ctx context.Context, sender CommandSender, reqCmd *Command) (*Command, error) {
	if reqCmd == nil {
		return nil, errors.New("command cannot be nil")
	}

	if reqCmd.Status != "" {
		return nil, errors.New("invalid command status")
	}

	if reqCmd.ID == "" {
		return nil, errors.New("invalid command id")
	}

	c.pendingCommandsMutex.Lock()

	if _, ok := c.pendingCommands[reqCmd.ID]; ok {
		c.pendingCommandsMutex.Unlock()
		return nil, errors.New("the command id is already in use")
	}

	respChan := make(chan *Command, 1)
	c.pendingCommands[reqCmd.ID] = respChan
	c.pendingCommandsMutex.Unlock()

	err := sender.SendCommand(ctx, reqCmd)
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case respCmd := <-respChan:
		return respCmd, nil
	}
}

func (c *commandProcessorChannel) TrySubmitCommandResult(respCmd *Command) bool {
	if respCmd == nil {
		return false
	}

	c.pendingCommandsMutex.RLocker()
	defer c.pendingCommandsMutex.RUnlock()

	respChan, ok := c.pendingCommands[respCmd.ID]
	if !ok {
		return false
	}

	delete(c.pendingCommands, respCmd.ID)

	respChan <- respCmd
	return true
}

// ClientChannel implements the client-side communication channel in a Lime session.
type ClientChannel struct {
	channel
}

func NewClientChannel(transport Transport) *ClientChannel {
	return &ClientChannel{
		channel{transport: transport},
	}
}

// ReceiveSession receives a session from the remote node.
func (c *ClientChannel) ReceiveSession(ctx context.Context) (*Session, error) {
	ses, err := c.receiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("receive session failed: %w", err)
	}

	if ses.State == SessionStateEstablished {
		c.localNode = ses.To
		c.remoteNode = ses.From
	}

	c.sessionID = ses.ID
	c.state = ses.State

	if ses.State == SessionStateFinished || ses.State == SessionStateFailed {
		if err := c.transport.Close(ctx); err != nil {
			return nil, fmt.Errorf("closing the transport failed: %w", err)
		}
	}

	return ses, nil
}

// StartNewSession sends a new session envelope to the server and awaits for the response.
func (c *ClientChannel) StartNewSession(ctx context.Context) (*Session, error) {
	err := c.ensureState(SessionStateNew, "start new session")
	if err != nil {
		return nil, err
	}

	err = c.SendSession(ctx, &Session{State: SessionStateNew})
	if err != nil {
		return nil, fmt.Errorf("sending new session failed: %w", err)
	}

	ses, err := c.ReceiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on new session failed: %w", err)
	}

	return ses, nil
}

// NegotiateSession sends a negotiate session envelope to accept the session negotiation options and awaits for the server confirmation.
func (c *ClientChannel) NegotiateSession(ctx context.Context, compression SessionCompression, encryption SessionEncryption) (*Session, error) {
	err := c.ensureState(SessionStateNegotiating, "negotiate session")
	if err != nil {
		return nil, err
	}

	negSes := Session{
		EnvelopeBase: EnvelopeBase{
			ID: c.sessionID,
		},
		State:       SessionStateNegotiating,
		Compression: compression,
		Encryption:  encryption,
	}

	err = c.SendSession(ctx, &negSes)
	if err != nil {
		return nil, fmt.Errorf("sending negotiating session failed: %w", err)
	}

	ses, err := c.ReceiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on session negotiation failed: %w", err)
	}

	return ses, nil
}

// AuthenticateSession send a authenticate session envelope to the server to establish an authenticated session and awaits for the response.
func (c *ClientChannel) AuthenticateSession(ctx context.Context, auth Authentication, instance string) (*Session, error) {
	err := c.ensureState(SessionStateAuthenticating, "authenticate session")
	if err != nil {
		return nil, err
	}

	authSes := Session{
		EnvelopeBase: EnvelopeBase{
			ID: c.sessionID,
		},
		State:          SessionStateAuthenticating,
		Authentication: auth,
	}

	err = c.SendSession(ctx, &authSes)
	if err != nil {
		return nil, fmt.Errorf("sending authenticating session failed: %w", err)
	}

	ses, err := c.ReceiveSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("receiving on session authentication failed: %w", err)
	}

	return ses, nil
}

type ServerChannel struct {
	channel
}
