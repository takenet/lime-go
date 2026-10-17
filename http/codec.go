package http

import (
	"fmt"

	"github.com/takenet/lime-go/codec"
	lime "github.com/takenet/lime-go/protocol"
)

// decodeEnvelope decodes data into the concrete lime envelope type it
// represents, returning one of *lime.Message, *lime.Notification,
// *lime.RequestCommand or *lime.ResponseCommand.
func decodeEnvelope(data []byte) (any, error) {
	envelope, err := codec.UnmarshalEnvelope(data)
	if err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	switch envelope.(type) {
	case *lime.Message, *lime.Notification, *lime.RequestCommand, *lime.ResponseCommand:
		return envelope, nil
	default:
		return nil, fmt.Errorf("decode envelope: unsupported envelope type %T", envelope)
	}
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
