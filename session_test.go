package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSession_MarshalJSON_New(t *testing.T) {
	// Arrange
	s := Session{}
	s.State = SessionNew

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"state":"new"}`, string(b))
}
func TestSession_MarshalJSON_Negotiating(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = &Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionNegotiating
	s.EncryptionOptions = []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"negotiating","encryptionOptions":["none","tls"]}`, string(b))
}
