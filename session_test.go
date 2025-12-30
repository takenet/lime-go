package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createSession() *Session {
	s := Session{}
	s.ID = testCommandID
	s.From = Node{}
	s.From.Name = "postmaster"
	s.From.Domain = testDomain
	s.From.Instance = testServerInstance
	s.To = Node{}
	s.To.Name = "golang"
	s.To.Domain = testDomain
	s.To.Instance = "default"
	s.State = SessionStateEstablished
	return &s
}

func TestSessionMarshalJSONNew(t *testing.T) {
	// Arrange
	s := Session{}
	s.State = SessionStateNew

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, `{"state":"new"}`, string(b))
}

func TestSessionMarshalJSONNegotiatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateNegotiating
	s.EncryptionOptions = []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}
	s.CompressionOptions = []SessionCompression{SessionCompressionNone}

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"negotiating","encryptionOptions":["none","tls"],"compressionOptions":["none"]}`, string(b))
}

func TestSessionMarshalJSONNegotiating(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.State = SessionStateNegotiating
	s.Encryption = SessionEncryptionTLS
	s.Compression = SessionCompressionNone

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","state":"negotiating","encryption":"tls","compression":"none"}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingOptions(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SchemeOptions = []AuthenticationScheme{AuthenticationSchemeGuest, AuthenticationSchemePlain, AuthenticationSchemeTransport, AuthenticationSchemeKey, AuthenticationSchemeExternal}

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"authenticating","schemeOptions":["guest","plain","transport","key","external"]}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingPlain(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&PlainAuthentication{Password: testPassword})

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"plain","authentication":{"password":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingGuest(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&GuestAuthentication{})

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"guest","authentication":{}}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingKey(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&KeyAuthentication{Key: testPassword})

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"key","authentication":{"key":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingTransport(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&TransportAuthentication{})

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"transport","authentication":{}}`, string(b))
}

func TestSessionMarshalJSONAuthenticatingExternal(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateAuthenticating
	s.SetAuthentication(&ExternalAuthentication{Token: "HePX3PtLNJ1hDubBJmxHGAfQnTczpeze", Issuer: "auth.limeprotocol.org"})

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"external","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`, string(b))
}

func TestSessionMarshalJSONEstablished(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.State = SessionStateEstablished

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"established"}`, string(b))
}

func TestSessionMarshalJSONFinishing(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.To = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.State = SessionStateFinishing

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"finishing"}`, string(b))
}

func TestSessionMarshalJSONFinished(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.State = SessionStateFinished

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"finished"}`, string(b))
}

func TestSessionMarshalJSONFailed(t *testing.T) {
	// Arrange
	s := Session{}
	s.ID = testCommandID
	s.From = Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: testServerInstance,
	}
	s.To = Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	}
	s.Reason = &Reason{
		Code:        13,
		Description: "The session authentication failed",
	}
	s.State = SessionStateFailed

	// Act
	b, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"failed","reason":{"code":13,"description":"The session authentication failed"}}`, string(b))
}

func TestSessionUnmarshalJSONNew(t *testing.T) {
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

func TestSessionUnmarshalJSONNegotiatingOptions(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"negotiating","encryptionOptions":["none","tls"],"compressionOptions":["none"]}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateNegotiating, s.State)
	assert.Equal(t, []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}, s.EncryptionOptions)
	assert.Equal(t, []SessionCompression{SessionCompressionNone}, s.CompressionOptions)
}

func TestSessionUnmarshalJSONNegotiating(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","state":"negotiating","encryption":"tls","compression":"none"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Zero(t, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateNegotiating, s.State)
	assert.Equal(t, SessionEncryptionTLS, s.Encryption)
	assert.Equal(t, SessionCompressionNone, s.Compression)
}

func TestSessionUnmarshalJSONAuthenticatingOptions(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","state":"authenticating","schemeOptions":["guest","plain","transport","key","external"]}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.From)
	assert.Zero(t, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, []AuthenticationScheme{AuthenticationSchemeGuest, AuthenticationSchemePlain, AuthenticationSchemeTransport, AuthenticationSchemeKey, AuthenticationSchemeExternal}, s.SchemeOptions)
}

func TestSessionUnmarshalJSONAuthenticatingPlain(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"plain","authentication":{"password":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemePlain, s.Scheme)
	assert.IsType(t, &PlainAuthentication{}, s.Authentication)
	a := s.Authentication.(*PlainAuthentication)
	assert.Equal(t, testPassword, a.Password)
}

func TestSessionUnmarshalJSONAuthenticatingGuest(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"guest","authentication":{}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeGuest, s.Scheme)
	assert.IsType(t, &GuestAuthentication{}, s.Authentication)
}

func TestSessionUnmarshalJSONAuthenticatingKey(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"key","authentication":{"key":"bXl2ZXJ5c2VjcmV0cGFzc3dvcmQ="}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeKey, s.Scheme)
	assert.IsType(t, &KeyAuthentication{}, s.Authentication)
	a := s.Authentication.(*KeyAuthentication)
	assert.Equal(t, testPassword, a.Key)

}

func TestSessionUnmarshalJSONAuthenticatingTransport(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"transport","authentication":{}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeTransport, s.Scheme)
	assert.IsType(t, &TransportAuthentication{}, s.Authentication)
}

func TestSessionUnmarshalJSONAuthenticatingExternal(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"external","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateAuthenticating, s.State)
	assert.Equal(t, AuthenticationSchemeExternal, s.Scheme)
	assert.IsType(t, &ExternalAuthentication{}, s.Authentication)
	a := s.Authentication.(*ExternalAuthentication)
	assert.Equal(t, "HePX3PtLNJ1hDubBJmxHGAfQnTczpeze", a.Token)
	assert.Equal(t, "auth.limeprotocol.org", a.Issuer)
}

func TestSessionUnmarshalJSONAuthenticatingUnknown(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"authenticating","scheme":"unknown","authentication":{"token":"HePX3PtLNJ1hDubBJmxHGAfQnTczpeze","issuer":"auth.limeprotocol.org"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)

	// Assert
	assert.Error(t, err)
}

func TestSessionUnmarshalJSONEstablished(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"established"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.To)
	assert.Equal(t, SessionStateEstablished, s.State)
}

func TestSessionUnmarshalJSONFinishing(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"golang@limeprotocol.org/default","to":"postmaster@limeprotocol.org/#server1","state":"finishing"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.To)
	assert.Equal(t, SessionStateFinishing, s.State)
}

func TestSessionUnmarshalJSONFinished(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"finished"}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.To)
	assert.Equal(t, SessionStateFinished, s.State)
}

func TestSessionUnmarshalJSONFailed(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","state":"failed","reason":{"code":13,"description":"The session authentication failed"}}`)
	var s Session

	// Act
	err := json.Unmarshal(j, &s)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, s.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, s.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, s.To)
	assert.Equal(t, SessionStateFailed, s.State)
	assert.Equal(t, Reason{13, "The session authentication failed"}, *s.Reason)
}

func TestPlainAuthenticationSetPasswordAsBase64(t *testing.T) {
	// Arrange
	auth := &PlainAuthentication{}
	plainPassword := "mySecretPassword123"

	// Act
	auth.SetPasswordAsBase64(plainPassword)

	// Assert
	assert.NotEmpty(t, auth.Password)
	decoded, err := auth.GetPasswordFromBase64()
	assert.NoError(t, err)
	assert.Equal(t, plainPassword, decoded)
}

func TestPlainAuthenticationGetPasswordFromBase64(t *testing.T) {
	// Arrange
	auth := &PlainAuthentication{
		Password: "bXlTZWNyZXRQYXNzd29yZDEyMw==", // base64 of "mySecretPassword123"
	}

	// Act
	decoded, err := auth.GetPasswordFromBase64()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "mySecretPassword123", decoded)
}

func TestPlainAuthenticationGetPasswordFromBase64InvalidBase64(t *testing.T) {
	// Arrange
	auth := &PlainAuthentication{
		Password: "not-valid-base64!!!",
	}

	// Act
	_, err := auth.GetPasswordFromBase64()

	// Assert
	assert.Error(t, err)
}

func TestKeyAuthenticationSetKeyAsBase64(t *testing.T) {
	// Arrange
	auth := &KeyAuthentication{}
	plainKey := "mySecretKey456"

	// Act
	auth.SetKeyAsBase64(plainKey)

	// Assert
	assert.NotEmpty(t, auth.Key)
	decoded, err := auth.GetKeyFromBase64()
	assert.NoError(t, err)
	assert.Equal(t, plainKey, decoded)
}

func TestKeyAuthenticationGetKeyFromBase64(t *testing.T) {
	// Arrange
	auth := &KeyAuthentication{
		Key: "bXlTZWNyZXRLZXk0NTY=", // base64 of "mySecretKey456"
	}

	// Act
	decoded, err := auth.GetKeyFromBase64()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "mySecretKey456", decoded)
}

func TestKeyAuthenticationGetKeyFromBase64InvalidBase64(t *testing.T) {
	// Arrange
	auth := &KeyAuthentication{
		Key: "not-valid-base64!!!",
	}

	// Act
	_, err := auth.GetKeyFromBase64()

	// Assert
	assert.Error(t, err)
}
