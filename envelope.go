package lime

import "github.com/google/uuid"

// Base struct to all communication envelopes.
type Envelope struct {
	// The envelope identifier
	ID string `json:"id,omitempty"`
	// The identifier of the sender node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope was originated by the remote party.
	From *Node `json:"from,omitempty"`
	// The delegation node. Its an acronym for 'per procurationem'.
	// Identifier of a delegate node (a node that received a permission to send on behalf of another).
	// Allows a node to send an envelope on behalf of another identity.
	Pp *Node `json:"pp,omitempty"`
	// The identifier of the destination node of the envelope.
	// If a node receives an envelope without this value, it means that the envelope is addressed to itself.
	To *Node `json:"to,omitempty"`
	// Additional information to be delivered with the envelope.
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

// Generates a new unique envelope id.
func NewEnvelopeId() string {
	return uuid.New().String()
}
