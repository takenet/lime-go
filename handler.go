package lime

import (
	"context"
	"fmt"
)

type EnvelopeMux struct {
	msgHandlers []MessageHandler
	notHandlers []NotificationHandler
	cmdHandlers []CommandHandler
}

func (m *EnvelopeMux) ListenServer(ctx context.Context, c *ServerChannel) error {
	return m.listen(ctx, c.channel)

}

func (m *EnvelopeMux) ListenClient(ctx context.Context, c *ClientChannel) error {
	return m.listen(ctx, c.channel)
}

func (m *EnvelopeMux) listen(ctx context.Context, c *channel) error {
	if err := c.ensureEstablished("receive"); err != nil {
		return fmt.Errorf("envelope mux: %w", err)
	}

	for c.Established() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.SesChan():
			break
		case msg := <-c.MsgChan():
			m.HandleMessage(msg)
		case not := <-c.NotChan():
			m.HandleNotification(not)
		case cmd := <-c.CmdChan():
			m.HandleCommand(cmd)
		}
	}
	return nil
}

func (m *EnvelopeMux) HandleMessage(msg *Message) {
	for _, h := range m.msgHandlers {
		if h.Match(msg) {
			h.Handle(msg)
		}
	}
}

func (m *EnvelopeMux) HandleNotification(not *Notification) {
	for _, h := range m.notHandlers {
		if h.Match(not) {
			h.Handle(not)
		}
	}
}

func (m *EnvelopeMux) HandleCommand(cmd *Command) {
	for _, h := range m.cmdHandlers {
		if h.Match(cmd) {
			h.Handle(cmd)
		}
	}
}

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

type MessageHandler interface {
	Match(m *Message) bool
	Handle(m *Message)
}

type MessagePredicate func(m *Message) bool

type MessageHandlerFunc func(m *Message)

type messageHandler struct {
	predicate   MessagePredicate
	handlerFunc MessageHandlerFunc
}

func (h *messageHandler) Match(m *Message) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(m)
}

func (h *messageHandler) Handle(m *Message) {
	h.handlerFunc(m)
}

type NotificationHandler interface {
	Match(n *Notification) bool
	Handle(n *Notification)
}

type NotificationPredicate func(n *Notification) bool

type NotificationHandlerFunc func(n *Notification)

type notificationHandler struct {
	predicate   NotificationPredicate
	handlerFunc NotificationHandlerFunc
}

func (h *notificationHandler) Match(n *Notification) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(n)
}

func (h *notificationHandler) Handle(n *Notification) {
	h.handlerFunc(n)
}

type CommandHandler interface {
	Match(c *Command) bool
	Handle(c *Command)
}

type CommandPredicate func(c *Command) bool

type CommandHandlerFunc func(c *Command)

type commandHandler struct {
	predicate   CommandPredicate
	handlerFunc CommandHandlerFunc
}

func (h *commandHandler) Match(c *Command) bool {
	if h.predicate == nil {
		return true
	}
	return h.predicate(c)
}

func (h *commandHandler) Handle(c *Command) {
	h.handlerFunc(c)
}
