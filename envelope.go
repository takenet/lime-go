package lime

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// Envelope is the base struct to all protocol envelopes.
type Envelope struct {
	// ID is the envelope identifier
	ID string
	// From is the identifier of the sender node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope was originated by the remote party.
	From Node
	// PP is the delegation node. It's an acronym for 'per procurationem'.
	// Identifier of a delegate node (a node that received a permission To send on behalf of another).
	// Allows a node to send an envelope on behalf of another identity.
	PP Node
	// To is the identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed To itself.
	To Node
	// Metadata holds additional information to be delivered with the envelope.
	Metadata map[string]string
}

func (env *Envelope) SetID(id string) *Envelope {
	env.ID = id
	return env
}

func (env *Envelope) SetNewEnvelopeID() *Envelope {
	env.ID = NewEnvelopeID()
	return env
}

func (env *Envelope) SetFrom(from Node) *Envelope {
	env.From = from
	return env
}

func (env *Envelope) SetFromString(s string) *Envelope {
	from := ParseNode(s)
	return env.SetFrom(from)
}

func (env *Envelope) SetTo(to Node) *Envelope {
	env.To = to
	return env
}

func (env *Envelope) SetToString(s string) *Envelope {
	to := ParseNode(s)
	return env.SetTo(to)
}

func (env *Envelope) SetPP(pp Node) *Envelope {
	env.PP = pp
	return env
}

func (env *Envelope) SetPPString(s string) *Envelope {
	pp := ParseNode(s)
	return env.SetPP(pp)
}

func (env *Envelope) SetMetadataKeyValue(key string, value string) *Envelope {
	if env.Metadata == nil {
		env.Metadata = make(map[string]string)
	}
	env.Metadata[key] = value
	return env
}

// Sender returns the envelope sender Node.
func (env *Envelope) Sender() Node {
	if env.PP != (Node{}) {
		return env.PP
	}
	return env.From
}

func (env *Envelope) toRawEnvelope() (*rawEnvelope, error) {
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

func (env *Envelope) populate(raw *rawEnvelope) error {
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

// NewEnvelopeID generates a new unique envelope ID.
func NewEnvelopeID() string {
	return uuid.New().String()
}

// envelope is the base interface for envelopes types.
type envelope interface {
	populate(raw *rawEnvelope) error
	toRawEnvelope() (*rawEnvelope, error)
}

// rawEnvelope is an intermediate type for marshalling.
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

func (re *rawEnvelope) envelopeType() (string, error) {
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

func (re *rawEnvelope) toEnvelope() (envelope, error) {
	var env envelope

	t, err := re.envelopeType()
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
