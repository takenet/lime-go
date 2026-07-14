package http

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/takenet/lime-go"
)

// sniffEnvelope is an intermediate type used only to determine the concrete
// envelope type carried by a raw JSON payload, mirroring the detection order
// used internally by the lime package (see envelope.go's
// rawEnvelope.envelopeType()). It intentionally only decodes the minimal set
// of discriminator fields, leaving the full unmarshalling to the concrete
// lime.Message/lime.Notification/lime.RequestCommand/lime.ResponseCommand
// types, which already know how to parse themselves.
type sniffEnvelope struct {
	Content *json.RawMessage `json:"content,omitempty"`
	Event   *string          `json:"event,omitempty"`
	Method  *string          `json:"method,omitempty"`
	URI     *string          `json:"uri,omitempty"`
	Status  *string          `json:"status,omitempty"`
}

// envelopeKind identifies the concrete LIME envelope type carried by an HTTP
// request or response body.
type envelopeKind string

const (
	envelopeKindMessage         envelopeKind = "message"
	envelopeKindNotification    envelopeKind = "notification"
	envelopeKindRequestCommand  envelopeKind = "requestCommand"
	envelopeKindResponseCommand envelopeKind = "responseCommand"
)

// detectEnvelopeKind determines the envelope kind carried by data, following
// the same precedence rules as lime.rawEnvelope.envelopeType(): a command is
// identified by the presence of a method, further split into request/response
// by the presence of a uri or a status; a notification is identified by the
// presence of an event; a message is identified by the presence of content.
func detectEnvelopeKind(data []byte) (envelopeKind, error) {
	var s sniffEnvelope
	if err := json.Unmarshal(data, &s); err != nil {
		return "", fmt.Errorf("detect envelope kind: %w", err)
	}

	if s.Method != nil {
		if s.URI != nil {
			return envelopeKindRequestCommand, nil
		}
		if s.Status != nil {
			return envelopeKindResponseCommand, nil
		}
	}
	if s.Event != nil {
		return envelopeKindNotification, nil
	}
	if s.Content != nil {
		return envelopeKindMessage, nil
	}

	return "", errors.New("could not determine the envelope kind")
}

// decodeEnvelope decodes data into the concrete lime envelope type it
// represents, returning one of *lime.Message, *lime.Notification,
// *lime.RequestCommand or *lime.ResponseCommand.
func decodeEnvelope(data []byte) (any, error) {
	kind, err := detectEnvelopeKind(data)
	if err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}

	var env any
	switch kind {
	case envelopeKindMessage:
		env = &lime.Message{}
	case envelopeKindNotification:
		env = &lime.Notification{}
	case envelopeKindRequestCommand:
		env = &lime.RequestCommand{}
	case envelopeKindResponseCommand:
		env = &lime.ResponseCommand{}
	default:
		return nil, fmt.Errorf("decode envelope: unsupported envelope kind %q", kind)
	}

	if err := json.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("decode envelope: unmarshal %v: %w", kind, err)
	}

	return env, nil
}

// envelopeContentType returns the LIME-specific media type that should be
// used as the Content-Type header when sending the given envelope.
func envelopeContentType(env any) (string, error) {
	switch env.(type) {
	case *lime.Message:
		return "application/vnd.lime.message+json", nil
	case *lime.Notification:
		return "application/vnd.lime.notification+json", nil
	case *lime.RequestCommand, *lime.ResponseCommand:
		return "application/vnd.lime.command+json", nil
	default:
		return "", fmt.Errorf("envelope content type: unsupported envelope type %T", env)
	}
}

// acceptableEnvelopeContentTypes are the Content-Type header values accepted
// for incoming envelope requests, matching the equivalent list used by
// lime-csharp's HttpMiddleware.
var acceptableEnvelopeContentTypes = map[string]bool{
	"application/json":                       true,
	"application/vnd.lime.message+json":      true,
	"application/vnd.lime.notification+json": true,
	"application/vnd.lime.command+json":      true,
}
