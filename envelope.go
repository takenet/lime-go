package lime

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
)

// Base interface for envelopes types.
type Envelope interface {
	getID() string
	getFrom() Node
	getPP() Node
	getTo() Node
	getMetadata() map[string]string
}

// Base struct to all communication envelopes.
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

func (e *EnvelopeBase) getID() string {
	return e.ID
}

func (e *EnvelopeBase) getFrom() Node {
	return e.From
}

func (e *EnvelopeBase) getPP() Node {
	return e.PP
}

func (e *EnvelopeBase) getTo() Node {
	return e.To
}

func (e *EnvelopeBase) getMetadata() map[string]string {
	return e.Metadata
}

func (e *EnvelopeBase) toWrapper() (EnvelopeBaseWrapper, error) {
	ew := EnvelopeBaseWrapper{}
	ew.ID = e.ID
	if e.From != (Node{}) {
		ew.From = &e.From
	}
	if e.PP != (Node{}) {
		ew.PP = &e.PP
	}
	if e.To != (Node{}) {
		ew.To = &e.To
	}

	return ew, nil
}

func (e *EnvelopeBase) populate(ew *EnvelopeBaseWrapper) error {
	if ew == nil || e == nil {
		return nil
	}
	e.ID = ew.ID
	e.Metadata = ew.Metadata
	if ew.From != nil {
		e.From = *ew.From
	}
	if ew.PP != nil {
		e.PP = *ew.PP
	}
	if ew.To != nil {
		e.To = *ew.To
	}

	return nil
}

// Wrapper for custom marshalling
type EnvelopeBaseWrapper struct {
	ID       string            `json:"id,omitempty"`
	From     *Node             `json:"from,omitempty"`
	PP       *Node             `json:"pp,omitempty"`
	To       *Node             `json:"to,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

func UnmarshalJSONMap(m map[string]*json.RawMessage) (Envelope, error) {
	var e Envelope

	if _, ok := m["method"]; ok {
		e = &Command{}
	}

	if _, ok := m["event"]; ok {
		e = &Notification{}
	}

	if _, ok := m["content"]; ok {
		e = &Message{}
	}

	if _, ok := m["state"]; ok {
		e = &Session{}
	}

	if e == nil {
		return nil, errors.New("could not determine the envelope type")
	}

	// TODO: This is inefficient since we are allocating twice for the envelope.
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, e)
	return e, err
}

// Represents a known reason for events occurred during the client-server
// interactions.
type Reason struct {
	// The reason code
	Code int `json:"code,omitempty"`
	// The reason description
	Description string `json:"description,omitempty"`
}

// Generates a new unique envelope ID.
func NewEnvelopeId() string {
	return uuid.New().String()
}
