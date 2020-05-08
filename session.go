package lime

import (
	"encoding/json"
	"fmt"
)

// Allows the configuration and establishment of the communication channel between nodes.
type Session struct {
	Envelope
	// Informs or changes the state of a session.
	// Only the server can change the session state, but the client can request
	// the state transition.
	State SessionState `json:"state"`
	// Encryption options provided by the server during the session negotiation.
	EncryptionOptions []SessionEncryption `json:"encryptionOptions,omitempty"`
	// The encryption option selected for the session.
	// This property is provided by the client in the  negotiation and by the
	// server in the confirmation after that.
	Encryption SessionEncryption `json:"encryption,omitempty"`
	// Compression options provided by the server during the session negotiation.
	CompressionOptions []SessionCompression `json:"compressionOptions,omitempty"`
	// The compression option selected for the session.
	// This property is provided by the client in the negotiation and by the
	// server in the confirmation after that.
	Compression SessionCompression `json:"compression,omitempty"`
	// List of available authentication schemas for session authentication provided
	// by the server.
	SchemeOptions []AuthenticationScheme `json:"schemeOptions,omitempty"`
	// The authentication scheme option selected for the session.
	// This property must be present if the property authentication is defined.
	Scheme AuthenticationScheme `json:"scheme,omitempty"`
	// RawAuthentication data, related To the selected schema.
	// Information like password sent by the client or roundtrip data sent by the server.
	Authentication Authentication `json:"authentication,omitempty"`
	// In cases where the client receives a session with failed state,
	// this property should provide more details about the problem.
	Reason *Reason `json:"reason,omitempty"`
}

func (s *Session) SetAuthentication(a Authentication) {
	s.Authentication = a
	s.Scheme = a.GetAuthenticationScheme()
}

//
//func (s *Session) UnmarshalJSON(b []byte) error {
//	var sessionMap map[string]json.RawMessage
//	err := json.Unmarshal(b, &sessionMap)
//	if err != nil {
//		return err
//	}
//	session := Session{}
//
//	for k, v := range sessionMap {
//		var ok bool
//		ok, err = session.Envelope.unmarshalJSONField(k, v)
//		if !ok {
//			ok, err = session.unmarshalJSONField(k, v)
//		}
//
//		if !ok {
//			return fmt.Errorf(`unknown session field '%v'`, k)
//		}
//
//		if err != nil {
//			return err
//		}
//	}
//
//	if v, ok := sessionMap["authentication"]; ok {
//		if session.Scheme == "" {
//			return errors.New("scheme is required when authentication is present")
//		}
//		factory, ok := authFactories[session.Scheme]
//		if !ok {
//			return fmt.Errorf(`unknown authentication scheme '%v'`, session.Scheme)
//		}
//		auth := factory()
//		err := json.Unmarshal(v, &auth)
//		if err != nil {
//			return err
//		}
//		session.Authentication = auth
//	}
//
//	*s = session
//	return nil
//}

func (s *Session) unmarshalJSONField(n string, v json.RawMessage) (bool, error) {
	switch n {
	case "state":
		err := json.Unmarshal(v, &s.State)
		return true, err
	case "encryption":
		err := json.Unmarshal(v, &s.Encryption)
		return true, err
	case "encryptionOptions":
		err := json.Unmarshal(v, &s.EncryptionOptions)
		return true, err
	case "compression":
		err := json.Unmarshal(v, &s.Compression)
		return true, err
	case "compressionOptions":
		err := json.Unmarshal(v, &s.CompressionOptions)
		return true, err
	case "scheme":
		err := json.Unmarshal(v, &s.Scheme)
		return true, err
	case "schemeOptions":
		err := json.Unmarshal(v, &s.SchemeOptions)
		return true, err
	case "reason":
		err := json.Unmarshal(v, &s.Reason)
		return true, err
	case "authentication":
		// authentication requires scheme To be present so should be handled outside this
		return true, nil
	}

	return false, nil
}

// Defines the supported session states
type SessionState string

const (
	// The session is new and doesn't exists an established context.
	// It is sent by a client node To start a session with a server.
	SessionStateNew = SessionState("new")
	// The server and the client are negotiating the session options,
	// like cryptography and compression.
	// The server sends To the client the options (if available) and
	// the client chooses the desired options. If there's no options
	// (for instance, if the connection is already encrypted or the
	// transport protocol doesn't support these options), the server
	// SHOULD skip the negotiation.
	SessionStateNegotiating = SessionState("negotiating")
	// The session is being authenticated. The server sends To
	// the client the available authentication schemes list and
	// the client must choose one and send the specific authentication
	// data. The authentication can occurs in multiple round trips,
	// according To the selected schema.
	SessionStateAuthenticating = SessionState("authenticating")
	// The session is active and it is possible To send and receive
	// messages and commands. The server sends this state
	// after the session was authenticated.
	SessionStateEstablished = SessionState("established")
	// The client node is requesting To the server To finish the session.
	SessionStateFinishing = SessionState("finishing")
	// The session was gracefully finished by the server.
	SessionStateFinished = SessionState("finished")
	// A problem occurred while the session was established, under
	// negotiation or authentication and it was closed by the server.
	// In this case, the property reason MUST be present To provide
	// more details about the problem.
	SessionStateFailed = SessionState("failed")
)

func (s SessionState) IsValid() error {
	switch s {
	case SessionStateNew, SessionStateNegotiating, SessionStateAuthenticating, SessionStateEstablished, SessionStateFinishing, SessionStateFinished, SessionStateFailed:
		return nil
	}

	return fmt.Errorf("invalid session state '%v'", s)
}

func (s SessionState) MarshalText() ([]byte, error) {
	err := s.IsValid()
	if err != nil {
		return []byte{}, err
	}
	return []byte(s), nil
}

func (s *SessionState) UnmarshalText(text []byte) error {
	state := SessionState(text)
	err := state.IsValid()
	if err != nil {
		return err
	}
	*s = state
	return nil
}

// Defines the valid session encryption values.
type SessionEncryption string

const (
	// The session is not encrypted.
	SessionEncryptionNone = SessionEncryption("none")
	// The session is encrypted by TLS (Transport Layer Security).
	SessionEncryptionTLS = SessionEncryption("tls")
)

type SessionCompression string

const (
	// The session is not compressed.
	SessionCompressionNone = SessionCompression("none")
	// The session is using the GZip algorithm for compression.
	SessionCompressionGzip = SessionCompression("gzip")
)

// Defines the valid authentication schemes values.
type AuthenticationScheme string

const (
	// The server doesn't requires a client credential, and provides a temporary
	// identity To the node. Some restriction may apply To guest sessions, like
	// the inability of sending some commands or other nodes may want To block
	// messages originated by guest identities.
	AuthenticationSchemeGuest = AuthenticationScheme("guest")
	// Username and password authentication.
	AuthenticationSchemePlain = AuthenticationScheme("plain")
	// Transport layer authentication.
	AuthenticationSchemeTransport = AuthenticationScheme("transport")
	// Key authentication.
	AuthenticationSchemeKey = AuthenticationScheme("key")
	// Third-party authentication.
	AuthenticationSchemeExternal = AuthenticationScheme("external")
)

var authFactories = map[AuthenticationScheme]func() Authentication{
	AuthenticationSchemePlain: func() Authentication {
		return &PlainAuthentication{}
	},
	AuthenticationSchemeKey: func() Authentication {
		return &KeyAuthentication{}
	},
	AuthenticationSchemeTransport: func() Authentication {
		return &TransportAuthentication{}
	},
	AuthenticationSchemeExternal: func() Authentication {
		return &ExternalAuthentication{}
	},
}

// Defines a session authentications scheme container
type Authentication interface {
	GetAuthenticationScheme() AuthenticationScheme
}

// Defines a plain authentication scheme, that uses a password for authentication.
// Should be used only with encrypted sessions.
type PlainAuthentication struct {
	// Base64 representation of the password
	Password string `json:"password"`
}

func (a *PlainAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemePlain
}

// Defines a plain authentication scheme, that uses a key for authentication.
// Should be used only with encrypted sessions.
type KeyAuthentication struct {
	// Base64 representation of the key
	Key string `json:"key"`
}

func (a *KeyAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeKey
}

// Defines a transport layer authentication scheme.
type TransportAuthentication struct {
}

func (a *TransportAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeTransport
}

// Defines a external authentication scheme, that uses third-party validation.
type ExternalAuthentication struct {
	// The authentication token on base64 representation.
	Token string `json:"token"`
	// The trusted token issuer.
	Issuer string `json:"issuer"`
}

func (a *ExternalAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeExternal
}
