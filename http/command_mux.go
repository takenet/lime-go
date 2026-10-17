package http

import (
	"context"
	"errors"
	"fmt"
	"sync"

	lime "github.com/takenet/lime-go/protocol"
)

// CommandPredicate selects request commands for a registered processor.
type CommandPredicate func(*lime.RequestCommand) bool

type commandRegistration struct {
	predicate CommandPredicate
	handler   CommandHandlerFunc
}

// CommandMux dispatches each request to the first matching processor.
type CommandMux struct {
	mu       sync.RWMutex
	handlers []commandRegistration
}

// Register adds a processor. A nil predicate matches every request.
func (m *CommandMux) Register(predicate CommandPredicate, handler CommandHandlerFunc) error {
	if m == nil {
		return errors.New("command mux is nil")
	}
	if handler == nil {
		return errors.New("command handler is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, commandRegistration{predicate: predicate, handler: handler})
	return nil
}

// Handle dispatches a request or returns a LIME failure when no processor matches.
func (m *CommandMux) Handle(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
	if m == nil {
		return nil, errors.New("command mux is nil")
	}
	if cmd == nil {
		return nil, errors.New("request command is nil")
	}

	m.mu.RLock()
	handlers := append([]commandRegistration(nil), m.handlers...)
	m.mu.RUnlock()

	for _, registration := range handlers {
		if registration.predicate != nil && !registration.predicate(cmd) {
			continue
		}
		response, err := registration.handler(ctx, cmd)
		if err != nil {
			return nil, fmt.Errorf("handle command: %w", err)
		}
		return response, nil
	}

	if cmd.Method == lime.CommandMethodObserve && cmd.ID == "" {
		return nil, nil
	}
	return cmd.FailureResponse(&lime.Reason{
		Code:        lime.ReasonCodeCommandResourceNotSupported,
		Description: "The requested resource is not supported",
	}), nil
}
