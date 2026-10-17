package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/takenet/lime-go/codec"
	lime "github.com/takenet/lime-go/protocol"
)

// MessageHandlerFunc processes an inbound message forwarded by Iris. It may
// optionally return a Notification to be written back synchronously as the
// HTTP response (e.g. a "received"/"consumed" receipt); a nil notification
// results in an empty "202 Accepted" response.
type MessageHandlerFunc func(ctx context.Context, msg *lime.Message) (*lime.Notification, error)

// NotificationHandlerFunc processes an inbound notification forwarded by
// Iris. Notifications are fire-and-forget: the HTTP response is always an
// empty "202 Accepted" when the handler succeeds.
type NotificationHandlerFunc func(ctx context.Context, not *lime.Notification) error

// CommandHandlerFunc processes an inbound request command forwarded by Iris.
// It must return a ResponseCommand except for observe commands without an ID,
// which are fire-and-forget and may return nil.
type CommandHandlerFunc func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error)

// Handler is an http.Handler that receives LIME envelopes forwarded by Iris
// (Blip's HTTP gateway) as webhook POST requests, authenticates them and
// dispatches them to the configured handler functions.
//
// A zero-value handler for an envelope kind results in an empty
// "202 Accepted" response for messages/notifications, or a "501 Not
// Implemented" response for commands (since a synchronous response cannot be
// produced without a CommandHandler).
type Handler struct {
	// LocalNode identifies this application node. It is used only for error
	// reporting/logging purposes; it does not need to be set for the handler
	// to function.
	LocalNode lime.Node
	// Authenticator validates the credentials sent in every request's
	// Authorization header. If nil, requests are accepted without
	// authentication (not recommended outside of local development/tests).
	Authenticator Authenticator

	MessageHandler      MessageHandlerFunc
	NotificationHandler NotificationHandlerFunc
	CommandHandler      CommandHandlerFunc
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := stripContentTypeParams(r.Header.Get("Content-Type"))
	if !acceptableEnvelopeContentTypes[contentType] {
		http.Error(w, fmt.Sprintf("unsupported content type %q", contentType), http.StatusUnsupportedMediaType)
		return
	}

	if _, err := h.authenticate(ctx, r); err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="lime"`)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}

	env, err := decodeEnvelope(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.dispatch(ctx, w, env)
}

func (h *Handler) authenticate(ctx context.Context, r *http.Request) (lime.Identity, error) {
	identity, scheme, credential, err := parseAuthorizationHeader(r.Header.Get("Authorization"))
	if err != nil {
		return lime.Identity{}, err
	}

	if h.Authenticator == nil {
		return identity, nil
	}

	ok, err := h.Authenticator.Authenticate(ctx, identity, scheme, credential)
	if err != nil {
		return lime.Identity{}, fmt.Errorf("authenticate: %w", err)
	}
	if !ok {
		return lime.Identity{}, errors.New("authenticate: invalid credentials")
	}

	return identity, nil
}

func (h *Handler) dispatch(ctx context.Context, w http.ResponseWriter, env any) {
	switch e := env.(type) {
	case *lime.Message:
		h.dispatchMessage(ctx, w, e)
	case *lime.Notification:
		h.dispatchNotification(ctx, w, e)
	case *lime.RequestCommand:
		h.dispatchCommand(ctx, w, e)
	default:
		http.Error(w, fmt.Sprintf("unsupported envelope type %T", env), http.StatusBadRequest)
	}
}

func (h *Handler) dispatchMessage(ctx context.Context, w http.ResponseWriter, msg *lime.Message) {
	if h.MessageHandler == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	not, err := h.MessageHandler(ctx, msg)
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	if not == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	writeEnvelopeResponse(w, http.StatusOK, not)
}

func (h *Handler) dispatchNotification(ctx context.Context, w http.ResponseWriter, not *lime.Notification) {
	if h.NotificationHandler == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if err := h.NotificationHandler(ctx, not); err != nil {
		writeHandlerError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) dispatchCommand(ctx context.Context, w http.ResponseWriter, cmd *lime.RequestCommand) {
	if h.CommandHandler == nil {
		http.Error(w, "no command handler configured", http.StatusNotImplemented)
		return
	}

	respCmd, err := h.CommandHandler(ctx, cmd)
	if err != nil {
		if cmd.Method == lime.CommandMethodObserve && cmd.ID == "" {
			writeHandlerError(w, err)
			return
		}
		writeEnvelopeResponse(w, http.StatusOK, commandFailureResponse(cmd, err))
		return
	}
	if respCmd == nil {
		if cmd.Method == lime.CommandMethodObserve && cmd.ID == "" {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		http.Error(w, "command handler returned a nil response", http.StatusInternalServerError)
		return
	}

	writeEnvelopeResponse(w, http.StatusOK, respCmd)
}

// writeHandlerError maps an application handler error to an HTTP response,
// honoring context deadline/cancellation errors distinctly from generic
// failures.
func writeHandlerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		http.Error(w, err.Error(), http.StatusGatewayTimeout)
	case errors.Is(err, context.Canceled):
		http.Error(w, err.Error(), http.StatusRequestTimeout)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeEnvelopeResponse(w http.ResponseWriter, statusCode int, env any) {
	contentType, err := envelopeContentType(env)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	envelope, ok := env.(lime.EnvelopeValue)
	if !ok {
		http.Error(w, fmt.Sprintf("unsupported envelope type %T", env), http.StatusInternalServerError)
		return
	}
	data, err := codec.MarshalEnvelope(envelope)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

// stripContentTypeParams removes any parameters (e.g. "; charset=utf-8") from
// a Content-Type header value.
func stripContentTypeParams(contentType string) string {
	for i, c := range contentType {
		if c == ';' {
			return contentType[:i]
		}
	}
	return contentType
}
