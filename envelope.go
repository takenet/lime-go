package lime

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

// Envelope is a base interface for envelopes types.
type Envelope interface {
	// GetID gets the envelope identifier
	GetID() string

	// GetFrom gets the identifier of the sender node of the envelope.
	GetFrom() Node

	// GetPP gets the delegation node. Its an acronym for 'per procurationem'.
	GetPP() Node

	// GetTo gets the identifier of the destination node of the envelope.
	GetTo() Node

	// GetMetadata gets additional information to be delivered with the envelope.
	GetMetadata() map[string]string

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
	// Allows a node To send an envelope on behalf of another identity.
	PP Node

	// The identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed To itself.
	To Node

	// Additional information to be delivered with the envelope.
	Metadata map[string]string
}

func (e *EnvelopeBase) GetID() string {
	return e.ID
}

func (e *EnvelopeBase) GetFrom() Node {
	return e.From
}

func (e *EnvelopeBase) GetPP() Node {
	return e.PP
}

func (e *EnvelopeBase) GetTo() Node {
	return e.To
}

func (e *EnvelopeBase) GetMetadata() map[string]string {
	return e.Metadata
}

func (e *EnvelopeBase) ToRawEnvelope() (*RawEnvelope, error) {
	raw := RawEnvelope{}
	raw.ID = e.ID
	if e.From != (Node{}) {
		raw.From = &e.From
	}
	if e.PP != (Node{}) {
		raw.PP = &e.PP
	}
	if e.To != (Node{}) {
		raw.To = &e.To
	}

	return &raw, nil
}

func (e *EnvelopeBase) Populate(raw *RawEnvelope) error {
	if raw == nil || e == nil {
		return nil
	}
	e.ID = raw.ID
	e.Metadata = raw.Metadata
	if raw.From != nil {
		e.From = *raw.From
	}
	if raw.PP != nil {
		e.PP = *raw.PP
	}
	if raw.To != nil {
		e.To = *raw.To
	}

	return nil
}

// Reason represents a known reason for events occurred during the client-server
// interactions.
type Reason struct {
	// Code The reason code
	Code int `json:"code,omitempty"`
	// Description The reason description
	Description string `json:"description,omitempty"`
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
	var e Envelope

	t, err := re.EnvelopeType()
	if err != nil {
		return nil, err
	}

	switch t {
	case "Command":
		e = &Command{}
		break
	case "Notification":
		e = &Notification{}
		break
	case "Message":
		e = &Message{}
		break
	case "Session":
		e = &Session{}
		break
	default:
		return nil, errors.New("unknown or unsupported envelope type")
	}

	if err := e.Populate(re); err != nil {
		return nil, err
	}

	return e, nil
}
