package lime

import (
	"context"
	"errors"
	"fmt"
)

type EnvelopeMux struct {
	msgHandlers []MessageHandler
	notHandlers []NotificationHandler
	cmdHandlers []CommandHandler
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
		case cmd, ok := <-c.CmdChan():
			if !ok {
				return errors.New("cmd chan: channel closed")
			}
			if err := m.handleCommand(ctx, cmd, c); err != nil {
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

func (m *EnvelopeMux) handleCommand(ctx context.Context, cmd *Command, s Sender) error {
	for _, h := range m.cmdHandlers {
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

func (m *EnvelopeMux) CommandHandlerFunc(predicate CommandPredicate, f CommandHandlerFunc) {
	m.CommandHandler(&commandHandler{
		predicate:   predicate,
		handlerFunc: f,
	})
}

func (m *EnvelopeMux) CommandHandler(handler CommandHandler) {
	m.cmdHandlers = append(m.cmdHandlers, handler)
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

// CommandHandler defines a handler for processing Command instances received from a channel.
type CommandHandler interface {
	// Match indicates if the specified Command should be handled by the instance.
	Match(cmd *Command) bool

	// Handle execute an action for the specified Command.
	// If this method returns an error, it signals to the channel listener to stop.
	Handle(ctx context.Context, cmd *Command, s Sender) error
}

// CommandPredicate defines an expression for checking if the specified Command satisfies a condition.
type CommandPredicate func(cmd *Command) bool

// CommandHandlerFunc defines an action to be executed to a Command.
type CommandHandlerFunc func(ctx context.Context, cmd *Command, s Sender) error

type commandHandler struct {
	predicate   CommandPredicate
	handlerFunc CommandHandlerFunc
}

func (h *commandHandler) Match(cmd *Command) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(cmd)
}

func (h *commandHandler) Handle(ctx context.Context, cmd *Command, s Sender) error {
	return h.handlerFunc(ctx, cmd, s)
}
