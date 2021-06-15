package lime

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

// Envelope is a base interface for envelopes types.
type Envelope interface {
	GetID() string                  // Gets the envelope identifier
	GetFrom() Node                  // Gets the identifier of the sender node of the envelope.
	GetPP() Node                    // Gets the delegation node. Its an acronym for 'per procurationem'.
	GetTo() Node                    // Gets the identifier of the destination node of the envelope.
	GetMetadata() map[string]string // Gets additional information to be delivered with the envelope.
	Populate(raw *RawEnvelope) error
	ToRawEnvelope() (*RawEnvelope, error)
}

// EnvelopeBase is a base struct to all communication envelopes.
type EnvelopeBase struct {
	// The envelope identifier
	ID string
	// The identifier of the sender node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope was originated by the remote party.
	From Node
	// The delegation node. Its an acronym for 'per procurationem'.
	// Identifier of a delegate node (a node that received a permission To send on behalf of another).
	// Allows a node to send an envelope on behalf of another identity.
	PP Node
	// The identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed To itself.
	To Node
	// Additional information to be delivered with the envelope.
	Metadata map[string]string
}

func (env *EnvelopeBase) GetID() string {
	return env.ID
}

func (env *EnvelopeBase) GetFrom() Node {
	return env.From
}

func (env *EnvelopeBase) GetPP() Node {
	return env.PP
}

func (env *EnvelopeBase) GetTo() Node {
	return env.To
}

func (env *EnvelopeBase) GetMetadata() map[string]string {
	return env.Metadata
}

func (env *EnvelopeBase) ToRawEnvelope() (*RawEnvelope, error) {
	raw := RawEnvelope{}
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

	return &raw, nil
}

func (env *EnvelopeBase) Populate(raw *RawEnvelope) error {
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

// NewEnvelopeId generates a new unique envelope ID.
func NewEnvelopeId() string {
	return uuid.New().String()
}

// RawEnvelope it is an intermediate type for marshalling.
type RawEnvelope struct {
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
	Uri      *LimeUri         `json:"uri,omitempty"`
	Resource *json.RawMessage `json:"resource,omitempty"`
	Status   *CommandStatus   `json:"status,omitempty"`

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

func (re *RawEnvelope) EnvelopeType() (string, error) {
	// Determine the envelope type
	if re.Method != nil {
		return "Command", nil
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

func (re *RawEnvelope) ToEnvelope() (Envelope, error) {
	var env Envelope

	t, err := re.EnvelopeType()
	if err != nil {
		return nil, err
	}

	switch t {
	case "Command":
		env = &Command{}
		break
	case "Notification":
		env = &Notification{}
		break
	case "Message":
		env = &Message{}
		break
	case "Session":
		env = &Session{}
		break
	default:
		return nil, errors.New("unknown or unsupported envelope type")
	}

	if err := env.Populate(re); err != nil {
		return nil, err
	}

	return env, nil
}
