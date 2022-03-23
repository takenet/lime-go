package lime

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
)

// Envelope is a base interface for envelopes types.
type Envelope interface {
	populate(raw *rawEnvelope) error
	toRawEnvelope() (*rawEnvelope, error)
}

// EnvelopeBase is a base struct to all communication envelopes.
type EnvelopeBase struct {
	// The envelope identifier
	ID string
	// The identifier of the sender node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope was originated by the remote party.
	From Node
	// The delegation node. It's an acronym for 'per procurationem'.
	// Identifier of a delegate node (a node that received a permission To send on behalf of another).
	// Allows a node to send an envelope on behalf of another identity.
	PP Node
	// The identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed To itself.
	To Node
	// Additional information to be delivered with the envelope.
	Metadata map[string]string
}

// Sender returns the envelope sender Node.
func (env *EnvelopeBase) Sender() Node {
	if env.PP == (Node{}) {
		return env.PP
	} else {
		return env.From
	}
}

func (env *EnvelopeBase) toRawEnvelope() (*rawEnvelope, error) {
	raw := rawEnvelope{}
	raw.ID = env.ID
	if env.From != (Node{}) {
		raw.From = &env.From
	}
	if env.PP != (Node{}) {
		raw.PP = &env.PP
	}
	if env.To != (Node{}) {
		raw.To = &env.To
	}
	raw.Metadata = env.Metadata

	return &raw, nil
}

func (env *EnvelopeBase) populate(raw *rawEnvelope) error {
	if raw == nil || env == nil {
		return nil
	}
	env.ID = raw.ID
	env.Metadata = raw.Metadata
	if raw.From != nil {
		env.From = *raw.From
	}
	if raw.PP != nil {
		env.PP = *raw.PP
	}
	if raw.To != nil {
		env.To = *raw.To
	}

	return nil
}

// Reason represents a known reason for events occurred during the client-server
// interactions.
type Reason struct {
	Code        int    `json:"code,omitempty"`        // The reason code
	Description string `json:"description,omitempty"` // The reason description
}

func (r Reason) String() string {
	return fmt.Sprintf("Code: %v - Description: %v", r.Code, r.Description)
}

// NewEnvelopeId generates a new unique envelope ID.
func NewEnvelopeId() string {
	return uuid.New().String()
}

// rawEnvelope it is an intermediate type for marshalling.
type rawEnvelope struct {
	// Common envelope properties

	ID       string            `json:"id,omitempty"`
	From     *Node             `json:"from,omitempty"`
	PP       *Node             `json:"pp,omitempty"`
	To       *Node             `json:"to,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// Shared properties

	Reason *Reason    `json:"reason,omitempty"` // Shared by Notification and Message
	Type   *MediaType `json:"type,omitempty"`   // Shared by Message and Command

	// Message properties

	Content *json.RawMessage `json:"content,omitempty"`

	// Notification properties

	Event *NotificationEvent `json:"event,omitempty"`

	// Command properties

	Method   *CommandMethod   `json:"method,omitempty"`
	Resource *json.RawMessage `json:"resource,omitempty"`

	// RequestCommand properties

	URI *URI `json:"uri,omitempty"`

	// ResponseCommand properties

	Status *CommandStatus `json:"status,omitempty"`

	// Session properties

	State              *SessionState          `json:"state,omitempty"`
	EncryptionOptions  []SessionEncryption    `json:"encryptionOptions,omitempty"`
	Encryption         *SessionEncryption     `json:"encryption,omitempty"`
	CompressionOptions []SessionCompression   `json:"compressionOptions,omitempty"`
	Compression        *SessionCompression    `json:"compression,omitempty"`
	SchemeOptions      []AuthenticationScheme `json:"schemeOptions,omitempty"`
	Scheme             *AuthenticationScheme  `json:"scheme,omitempty"`
	Authentication     *json.RawMessage       `json:"authentication,omitempty"`
}

func (re *rawEnvelope) EnvelopeType() (string, error) {
	// Determine the envelope type
	if re.Method != nil {
		if re.URI != nil {
			return "RequestCommand", nil
		}
		if re.Status != nil {
			return "ResponseCommand", nil
		}
	}
	if re.Event != nil {
		return "Notification", nil
	}
	if re.Content != nil {
		return "Message", nil
	}
	if re.State != nil {
		return "Session", nil
	}

	return "", errors.New("could not determine the envelope type")
}

func (re *rawEnvelope) ToEnvelope() (Envelope, error) {
	var env Envelope

	t, err := re.EnvelopeType()
	if err != nil {
		return nil, err
	}

	switch t {
	case "RequestCommand":
		env = &RequestCommand{}
	case "ResponseCommand":
		env = &ResponseCommand{}
	case "Notification":
		env = &Notification{}
	case "Message":
		env = &Message{}
	case "Session":
		env = &Session{}
	default:
		return nil, errors.New("unknown or unsupported envelope type")
	}

	if err := env.populate(re); err != nil {
		return nil, err
	}

	return env, nil
}
