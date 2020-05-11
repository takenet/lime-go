package lime

import (
	"encoding/json"
	"github.com/google/uuid"
)

// Base struct To all communication envelopes.
type Envelope struct {
	// The envelope identifier
	ID string
	// The identifier of the sender node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope was originated by the remote party.
	From Node
	// The delegation node. Its an acronym for 'per procurationem'.
	// Identifier of a delegate node (a node that received a permission To send on behalf of another).
	// Allows a node To send an envelope on behalf of another identity.
	Pp Node
	// The identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed To itself.
	To Node
	// Additional information To be delivered with the envelope.
	Metadata map[string]string
}

func (e *Envelope) toWrapper() (EnvelopeWrapper, error) {

	ew := EnvelopeWrapper{}
	ew.Id = e.ID
	if e.From != (Node{}) {
		ew.From = &e.From
	}
	if e.Pp != (Node{}) {
		ew.Pp = &e.Pp
	}
	if e.To != (Node{}) {
		ew.To = &e.To
	}

	return ew, nil
}

func (e *Envelope) populate(ew *EnvelopeWrapper) error {
	if ew == nil || e == nil {
		return nil
	}

	e.ID = ew.Id
	e.Metadata = ew.Metadata
	if ew.From != nil {
		e.From = *ew.From
	}
	if ew.Pp != nil {
		e.Pp = *ew.Pp
	}
	if ew.To != nil {
		e.To = *ew.To
	}

	return nil
}

func (e *Envelope) unmarshalJSONField(n string, v json.RawMessage) (bool, error) {
	switch n {
	// envelope fields
	case "id":
		err := json.Unmarshal(v, &e.ID)
		return true, err
	case "from":
		err := json.Unmarshal(v, &e.From)
		return true, err
	case "pp":
		err := json.Unmarshal(v, &e.Pp)
		return true, err
	case "to":
		err := json.Unmarshal(v, &e.To)
		return true, err
	case "metadata":
		err := json.Unmarshal(v, &e.Metadata)
		return true, err
	}
	return false, nil
}

// Wrapper for custom marshalling
type EnvelopeWrapper struct {
	Id       string            `json:"id,omitempty"`
	From     *Node             `json:"from,omitempty"`
	Pp       *Node             `json:"pp,omitempty"`
	To       *Node             `json:"to,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Represents a known reason for events occurred during the client-server
// interactions.
type Reason struct {
	// The reason code
	Code int `json:"code,omitempty"`
	// The reason description
	Description string `json:"description,omitempty"`
}

// Generates a new unique envelope Id.
func NewEnvelopeId() string {
	return uuid.New().String()
}
