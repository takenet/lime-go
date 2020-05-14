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
	assert.Equal(t, `{"state":"new"}`, string(b))
}

func TestSession_MarshalJSON_NegotiatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
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
	assert.Equal(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","state":"negotiating","encryption":"tls","compression":"none"}`, string(b))
}

func TestSession_MarshalJSON_AuthenticatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
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
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = Node{
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

func TestSession_MarshalJSON_AuthenticatingKey(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&KeyAuthentication{Key: "bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="})

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"key","authentication":{"key":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`, string(b))
}

func TestSession_MarshalJSON_AuthenticatingTransport(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&TransportAuthentication{})

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"transport","authentication":{}}`, string(b))
}

func TestSession_MarshalJSON_AuthenticatingExternal(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&ExternalAuthentication{Token: "HePX3PtLNJ1hDubBJmxHGAfQnTczpeze", Issuer: "auth.limeprotocol.org"})

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"external","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`, string(b))
}

func TestSession_MarshalJSON_Established(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.State = SessionStateEstablished

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"established"}`, string(b))
}

func TestSession_MarshalJSON_Finishing(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.State = SessionStateFinishing

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"finishing"}`, string(b))
}

func TestSession_MarshalJSON_Finished(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.State = SessionStateFinished

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"finished"}`, string(b))
}

func TestSession_MarshalJSON_Failed(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "#server1",
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "default",
	}
	s.Reason = Reason{
		Code:        13,
		Description: "The session authentication failed",
	}
	s.State = SessionStateFailed

	// Act
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"failed","reason":{"code":13,"description":"The session authentication failed"}}`, string(b))
}

func TestSession_UnmarshalJSON_New(t *testing.T) {
	// Arrange
	j := []byte(`{"state":"new"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Zero(t, s.ID)
	assert.Zero(t, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateNew, s.State)
}

func TestSession_UnmarshalJSON_NegotiatingOptions(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"negotiating","encryptionOptions":["none","tls"],"compressionOptions":["none"]}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateNegotiating, s.State)
	assert.Equal(t, []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}, s.EncryptionOptions)
	assert.Equal(t, []SessionCompression{SessionCompressionNone}, s.CompressionOptions)
}

func TestSession_UnmarshalJSON_Negotiating(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","state":"negotiating","encryption":"tls","compression":"none"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Zero(t, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateNegotiating, s.State)
	assert.Equal(t, SessionEncryptionTLS, s.Encryption)
	assert.Equal(t, SessionCompressionNone, s.Compression)
}

func TestSession_UnmarshalJSON_AuthenticatingOptions(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"authenticating","schemeOptions":["guest","plain","transport","key","external"]}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, []AuthenticationScheme{AuthenticationSchemeGuest, AuthenticationSchemePlain, AuthenticationSchemeTransport, AuthenticationSchemeKey, AuthenticationSchemeExternal}, s.SchemeOptions)
}

func TestSession_UnmarshalJSON_AuthenticatingPlain(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"plain","authentication":{"password":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemePlain, s.Scheme)
	assert.IsType(t, &PlainAuthentication{}, s.Authentication)
	a := s.Authentication.(*PlainAuthentication)
	assert.Equal(t, "bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ=", a.Password)
}

func TestSession_UnmarshalJSON_AuthenticatingKey(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"key","authentication":{"key":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeKey, s.Scheme)
	assert.IsType(t, &KeyAuthentication{}, s.Authentication)
	a := s.Authentication.(*KeyAuthentication)
	assert.Equal(t, "bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ=", a.Key)

}

func TestSession_UnmarshalJSON_AuthenticatingTransport(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"transport","authentication":{}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeTransport, s.Scheme)
	assert.IsType(t, &TransportAuthentication{}, s.Authentication)
}

func TestSession_UnmarshalJSON_AuthenticatingExternal(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"external","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeExternal, s.Scheme)
	assert.IsType(t, &ExternalAuthentication{}, s.Authentication)
	a := s.Authentication.(*ExternalAuthentication)
	assert.Equal(t, "HePX3PtLNJ1hDubBJmxHGAfQnTczpeze", a.Token)
	assert.Equal(t, "auth.limeprotocol.org", a.Issuer)
}

func TestSession_UnmarshalJSON_AuthenticatingUnknown(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"unknown","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)

	// Assert
	assert.Error(t, err)
}

func TestSession_UnmarshalJSON_Established(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"established"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.To)
	assert.Equal(t, SessionStateEstablished, s.State)
}

func TestSession_UnmarshalJSON_Finishing(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"finishing"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.To)
	assert.Equal(t, SessionStateFinishing, s.State)
}

func TestSession_UnmarshalJSON_Finished(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"finished"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.To)
	assert.Equal(t, SessionStateFinished, s.State)
}

func TestSession_UnmarshalJSON_Failed(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"failed","reason":{"code":13,"description":"The session authentication failed"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", s.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, s.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, s.To)
	assert.Equal(t, SessionStateFailed, s.State)
	assert.Equal(t, Reason{13, "The session authentication failed"}, s.Reason)
}
