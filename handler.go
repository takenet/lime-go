package lime

import (
	"context"
	"errors"
	"fmt"
)

type EnvelopeMux struct {
	msgHandlers     []MessageHandler
	notHandlers     []NotificationHandler
	reqCmdHandlers  []RequestCommandHandler
	respCmdHandlers []ResponseCommandHandler
}

func (m *EnvelopeMux) ListenServer(ctx context.Context, c *ServerChannel) error {
	if err := m.listen(ctx, c.channel); err != nil {
		return fmt.Errorf("listen server: %w", err)
	}
	return nil
}

func (m *EnvelopeMux) ListenClient(ctx context.Context, c *ClientChannel) error {
	if err := m.listen(ctx, c.channel); err != nil {
		return fmt.Errorf("listen client: %w", err)
	}
	return nil
}

func (m *EnvelopeMux) listen(ctx context.Context, c *channel) error {
	if err := c.ensureEstablished("receive"); err != nil {
		return err
	}

	for c.Established() && ctx.Err() == nil {
		ctx := sessionContext(ctx, c)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.RcvDone():
			return nil
		case msg, ok := <-c.MsgChan():
			if !ok {
				return errors.New("msg chan: channel closed")
			}
			if err := m.handleMessage(ctx, msg, c); err != nil {
				return err
			}
		case not, ok := <-c.NotChan():
			if !ok {
				return errors.New("not chan: channel closed")
			}
			if err := m.handleNotification(ctx, not); err != nil {
				return err
			}
		case reqCmd, ok := <-c.ReqCmdChan():
			if !ok {
				return errors.New("req cmd chan: channel closed")
			}
			if err := m.handleRequestCommand(ctx, reqCmd, c); err != nil {
				return err
			}
		case respCmd, ok := <-c.RespCmdChan():
			if !ok {
				return errors.New("resp cmd chan: channel closed")
			}
			if err := m.handleResponseCommand(ctx, respCmd, c); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}

func (m *EnvelopeMux) handleMessage(ctx context.Context, msg *Message, s Sender) error {
	for _, h := range m.msgHandlers {
		if !h.Match(msg) {
			continue
		}
		if err := h.Handle(ctx, msg, s); err != nil {
			return fmt.Errorf("handle message: %w", err)
		}
		break
	}
	return nil
}

func (m *EnvelopeMux) handleNotification(ctx context.Context, not *Notification) error {
	for _, h := range m.notHandlers {
		if !h.Match(not) {
			continue
		}
		if err := h.Handle(ctx, not); err != nil {
			return fmt.Errorf("handle notification: %w", err)
		}
		break
	}
	return nil
}

func (m *EnvelopeMux) handleRequestCommand(ctx context.Context, cmd *RequestCommand, s Sender) error {
	for _, h := range m.reqCmdHandlers {
		if !h.Match(cmd) {
			continue
		}
		if err := h.Handle(ctx, cmd, s); err != nil {
			return fmt.Errorf("handle command: %w", err)
		}
		break
	}
	return nil
}

func (m *EnvelopeMux) handleResponseCommand(ctx context.Context, cmd *ResponseCommand, s Sender) error {
	for _, h := range m.respCmdHandlers {
		if !h.Match(cmd) {
			continue
		}
		if err := h.Handle(ctx, cmd, s); err != nil {
			return fmt.Errorf("handle command: %w", err)
		}
		break
	}
	return nil
}

// MessageHandlerFunc allows the definition of a function for handling received messages that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (m *EnvelopeMux) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) {
	m.MessageHandler(&messageHandler{
		predicate:   predicate,
		handlerFunc: f,
	})
}

func (m *EnvelopeMux) MessageHandler(handler MessageHandler) {
	m.msgHandlers = append(m.msgHandlers, handler)
}

func (m *EnvelopeMux) NotificationHandlerFunc(predicate NotificationPredicate, f NotificationHandlerFunc) {
	m.NotificationHandler(&notificationHandler{
		predicate:   predicate,
		handlerFunc: f,
	})
}

func (m *EnvelopeMux) NotificationHandler(handler NotificationHandler) {
	m.notHandlers = append(m.notHandlers, handler)
}

func (m *EnvelopeMux) RequestCommandHandlerFunc(predicate RequestCommandPredicate, f RequestCommandHandlerFunc) {
	m.RequestCommandHandler(&requestCommandHandler{
		predicate:   predicate,
		handlerFunc: f,
	})
}

func (m *EnvelopeMux) RequestCommandHandler(handler RequestCommandHandler) {
	m.reqCmdHandlers = append(m.reqCmdHandlers, handler)
}

func (m *EnvelopeMux) ResponseCommandHandlerFunc(predicate ResponseCommandPredicate, f ResponseCommandHandlerFunc) {
	m.ResponseCommandHandler(&responseCommandHandler{
		predicate:   predicate,
		handlerFunc: f,
	})
}

func (m *EnvelopeMux) ResponseCommandHandler(handler ResponseCommandHandler) {
	m.respCmdHandlers = append(m.respCmdHandlers, handler)
}

// MessageHandler defines a handler for processing Message instances received from a channel.
type MessageHandler interface {
	// Match indicates if the specified Message should be handled by the instance.
	Match(msg *Message) bool

	// Handle execute an action for the specified Message.
	// If this method returns an error, it signals to the channel listener to stop.
	Handle(ctx context.Context, msg *Message, s Sender) error
}

// MessagePredicate defines an expression for checking if the specified Message satisfies a condition.
type MessagePredicate func(msg *Message) bool

// MessageHandlerFunc defines an action to be executed to a Message.
type MessageHandlerFunc func(ctx context.Context, msg *Message, s Sender) error

type messageHandler struct {
	predicate   MessagePredicate
	handlerFunc MessageHandlerFunc
}

func (h *messageHandler) Match(msg *Message) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(msg)
}

func (h *messageHandler) Handle(ctx context.Context, msg *Message, s Sender) error {
	return h.handlerFunc(ctx, msg, s)
}

// NotificationHandler defines a handler for processing Notification instances received from a channel.
type NotificationHandler interface {
	// Match indicates if the specified Notification should be handled by the instance.
	Match(not *Notification) bool

	// Handle execute an action for the specified Notification.
	// If this method returns an error, it signals to the channel listener to stop.
	Handle(ctx context.Context, not *Notification) error
}

// NotificationPredicate defines an expression for checking if the specified Notification satisfies a condition.
type NotificationPredicate func(not *Notification) bool

// NotificationHandlerFunc defines an action to be executed to a Notification.
type NotificationHandlerFunc func(ctx context.Context, not *Notification) error

type notificationHandler struct {
	predicate   NotificationPredicate
	handlerFunc NotificationHandlerFunc
}

func (h *notificationHandler) Match(not *Notification) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(not)
}

func (h *notificationHandler) Handle(ctx context.Context, not *Notification) error {
	return h.handlerFunc(ctx, not)
}

// RequestCommandHandler defines a handler for processing Command instances received from a channel.
type RequestCommandHandler interface {
	// Match indicates if the specified RequestCommand should be handled by the instance.
	Match(cmd *RequestCommand) bool

	// Handle execute an action for the specified RequestCommand.
	// If this method returns an error, it signals to the channel listener to stop.
	Handle(ctx context.Context, cmd *RequestCommand, s Sender) error
}

// RequestCommandPredicate defines an expression for checking if the specified RequestCommand satisfies a condition.
type RequestCommandPredicate func(cmd *RequestCommand) bool

// RequestCommandHandlerFunc defines an action to be executed to a RequestCommand.
type RequestCommandHandlerFunc func(ctx context.Context, cmd *RequestCommand, s Sender) error

type requestCommandHandler struct {
	predicate   RequestCommandPredicate
	handlerFunc RequestCommandHandlerFunc
}

func (h *requestCommandHandler) Match(cmd *RequestCommand) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(cmd)
}

func (h *requestCommandHandler) Handle(ctx context.Context, cmd *RequestCommand, s Sender) error {
	return h.handlerFunc(ctx, cmd, s)
}

// ResponseCommandHandler defines a handler for processing Command instances received from a channel.
type ResponseCommandHandler interface {
	// Match indicates if the specified ResponseCommand should be handled by the instance.
	Match(cmd *ResponseCommand) bool

	// Handle execute an action for the specified ResponseCommand.
	// If this method returns an error, it signals to the channel listener to stop.
	Handle(ctx context.Context, cmd *ResponseCommand, s Sender) error
}

// ResponseCommandPredicate defines an expression for checking if the specified ResponseCommand satisfies a condition.
type ResponseCommandPredicate func(cmd *ResponseCommand) bool

// ResponseCommandHandlerFunc defines an action to be executed to a ResponseCommand.
type ResponseCommandHandlerFunc func(ctx context.Context, cmd *ResponseCommand, s Sender) error

type responseCommandHandler struct {
	predicate   ResponseCommandPredicate
	handlerFunc ResponseCommandHandlerFunc
}

func (h *responseCommandHandler) Match(cmd *ResponseCommand) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(cmd)
}

func (h *responseCommandHandler) Handle(ctx context.Context, cmd *ResponseCommand, s Sender) error {
	return h.handlerFunc(ctx, cmd, s)
}
