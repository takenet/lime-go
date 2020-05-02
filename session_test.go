package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSession_MarshalJSON_New(t *testing.T) {
	// Arrange
	s := Session{}
	s.State = SessionStateNew

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"state":"new"}`, string(b))
}

func TestSession_MarshalJSON_NegotiatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = &Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateNegotiating
	s.EncryptionOptions = []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}
	s.CompressionOptions = []SessionCompression{SessionCompressionNone}

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"negotiating","encryptionOptions":["none","tls"],"compressionOptions":["none"]}`, string(b))
}

func TestSession_MarshalJSON_Negotiating(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.State = SessionStateNegotiating
	s.Encryption = SessionEncryptionTLS
	s.Compression = SessionCompressionNone

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","state":"negotiating","encryption":"tls","compression":"none"}`, string(b))
}

func TestSession_MarshalJSON_AuthenticatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = &Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateAuthenticating
	s.SchemeOptions = []AuthenticationScheme{AuthenticationSchemeGuest, AuthenticationSchemePlain, AuthenticationSchemeTransport, AuthenticationSchemeKey, AuthenticationSchemeExternal}

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"authenticating","schemeOptions":["guest","plain","transport","key","external"]}`, string(b))
}

func TestSession_MarshalJSON_AuthenticatingPlain(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = &Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = &Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&PlainAuthentication{Password: "bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="})

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"plain","authentication":{"password":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`, string(b))
}
