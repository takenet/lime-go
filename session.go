package lime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// Session Allows the configuration and establishment of the communication channel between nodes.
type Session struct {
	EnvelopeBase
	// State Informs or changes the state of a session.
	// Only the server can change the session state, but the client can request
	// the state transition.
	State SessionState
	// EncryptionOptions Options provided by the server during the session negotiation.
	EncryptionOptions []SessionEncryption
	// Encryption The encryption option selected for the session.
	// This property is provided by the client in the  negotiation and by the
	// server in the confirmation after that.
	Encryption SessionEncryption
	// Compression options provided by the server during the session negotiation.
	CompressionOptions []SessionCompression
	// The compression option selected for the session.
	// This property is provided by the client in the negotiation and by the
	// server in the confirmation after that.
	Compression SessionCompression
	// List of available authentication schemas for session authentication provided
	// by the server.
	SchemeOptions []AuthenticationScheme
	// The authentication scheme option selected for the session.
	// This property must be present if the property authentication is defined.
	Scheme AuthenticationScheme
	// RawAuthentication data, related To the selected schema.
	// Information like password sent by the client or roundtrip data sent by the server.
	Authentication Authentication
	// In cases where the client receives a session with failed state,
	// this property should provide more details about the problem.
	Reason *Reason
}

func (s *Session) SetAuthentication(a Authentication) {
	s.Authentication = a
	s.Scheme = a.GetAuthenticationScheme()
}

func (s *Session) MarshalJSON() ([]byte, error) {
	raw, err := s.toRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (s *Session) UnmarshalJSON(b []byte) error {
	raw := rawEnvelope{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	session := Session{}
	err = session.populate(&raw)
	if err != nil {
		return err
	}

	*s = session
	return nil
}

func (s *Session) toRawEnvelope() (*rawEnvelope, error) {
	raw, err := s.EnvelopeBase.toRawEnvelope()
	if err != nil {
		return nil, err
	}

	if s.Authentication != nil {
		b, err := json.Marshal(s.Authentication)
		if err != nil {
			return nil, err
		}
		a := json.RawMessage(b)
		raw.Authentication = &a

		if s.Scheme != "" {
			raw.Scheme = &s.Scheme
		}
	}

	if s.State != "" {
		raw.State = &s.State
	}
	raw.EncryptionOptions = s.EncryptionOptions
	if s.Encryption != "" {
		raw.Encryption = &s.Encryption
	}
	raw.CompressionOptions = s.CompressionOptions
	if s.Compression != "" {
		raw.Compression = &s.Compression
	}
	raw.SchemeOptions = s.SchemeOptions
	if s.Scheme != "" {
		raw.Scheme = &s.Scheme
	}

	raw.Reason = s.Reason

	return raw, nil
}

func (s *Session) populate(raw *rawEnvelope) error {
	err := s.EnvelopeBase.populate(raw)
	if err != nil {
		return err
	}

	// Create the auth type instance and unmarshal the json to it
	if raw.Authentication != nil {
		if raw.Scheme == nil {
			return errors.New("session scheme is required when authentication is present")
		}

		factory, ok := authFactories[*raw.Scheme]
		if !ok {
			return fmt.Errorf(`unknown authentication scheme '%v'`, raw.Scheme)
		}
		a := factory()
		err := json.Unmarshal(*raw.Authentication, &a)
		if err != nil {
			return err
		}

		s.Authentication = a
		s.Scheme = *raw.Scheme
	}

	if raw.State == nil {
		return errors.New("session state is required")
	}

	s.State = *raw.State
	s.EncryptionOptions = raw.EncryptionOptions
	if raw.Encryption != nil {
		s.Encryption = *raw.Encryption
	}
	s.CompressionOptions = raw.CompressionOptions
	if raw.Compression != nil {
		s.Compression = *raw.Compression
	}
	s.SchemeOptions = raw.SchemeOptions
	if raw.Scheme != nil {
		s.Scheme = *raw.Scheme
	}

	s.Reason = raw.Reason

	return nil
}

// SessionState Defines the supported session states
type SessionState string

const (
	// SessionStateNew indicates that the session is new and doesn't exist an established context.
	// It is sent by a client node To start a session with a server.
	SessionStateNew = SessionState("new")
	// SessionStateNegotiating indicates that the server and the client are negotiating the session options,
	// like cryptography and compression.
	// The server sends To the client the options (if available) and
	// the client chooses the desired options. If there's no options
	// (for instance, if the connection is already encrypted or the
	// transport protocol doesn't support these options), the server
	// SHOULD skip the negotiation.
	SessionStateNegotiating = SessionState("negotiating")
	// SessionStateAuthenticating indicates that the session is being authenticated. The server sends To
	// the client the available authentication schemes list and
	// the client must choose one and send the specific authentication
	// data. The authentication can occur in multiple round trips,
	// according To the selected schema.
	SessionStateAuthenticating = SessionState("authenticating")
	// SessionStateEstablished indicates that the session is active, and it is possible To send and receive
	// messages and commands. The server sends this state
	// after the session was authenticated.
	SessionStateEstablished = SessionState("established")
	// SessionStateFinishing indicates that the client node is requesting to the server to finish the session.
	SessionStateFinishing = SessionState("finishing")
	// SessionStateFinished indicates that the session was gracefully finished by the server.
	SessionStateFinished = SessionState("finished")
	// SessionStateFailed indicates that a problem occurred while the session was established, under
	// negotiation or authentication, and it was closed by the server.
	// In this case, the property reason MUST be present to provide
	// more details about the problem.
	SessionStateFailed = SessionState("failed")
)

func (s SessionState) Validate() error {
	switch s {
	case SessionStateNew, SessionStateNegotiating, SessionStateAuthenticating, SessionStateEstablished, SessionStateFinishing, SessionStateFinished, SessionStateFailed:
		return nil
	}

	return fmt.Errorf("invalid session state '%v'", s)
}

func (s SessionState) Step() int {
	switch s {
	case SessionStateNew:
		return 0
	case SessionStateNegotiating:
		return 1
	case SessionStateAuthenticating:
		return 2
	case SessionStateEstablished:
		return 3
	case SessionStateFinishing:
		return 4
	case SessionStateFinished:
		return 5
	case SessionStateFailed:
		return 6
	}
	return -1
}

func (s SessionState) MarshalText() ([]byte, error) {
	err := s.Validate()
	if err != nil {
		return []byte{}, err
	}
	return []byte(s), nil
}

func (s *SessionState) UnmarshalText(text []byte) error {
	state := SessionState(text)
	err := state.Validate()
	if err != nil {
		return err
	}
	*s = state
	return nil
}

// SessionEncryption Defines the valid session encryption values.
type SessionEncryption string

const (
	// SessionEncryptionNone The session is not encrypted.
	SessionEncryptionNone = SessionEncryption("none")
	// SessionEncryptionTLS The session is encrypted by TLS (Transport Layer Security).
	SessionEncryptionTLS = SessionEncryption("tls")
)

// SessionCompression Defines the valid session compression values.
type SessionCompression string

const (
	// SessionCompressionNone The session is not compressed.
	SessionCompressionNone = SessionCompression("none")
	// SessionCompressionGzip The session is using the GZip algorithm for compression.
	SessionCompressionGzip = SessionCompression("gzip")
)

// AuthenticationScheme Defines the valid authentication schemes values.
type AuthenticationScheme string

const (
	// AuthenticationSchemeGuest The server doesn't require a client credential, and provides a temporary
	// identity to the node. Some restriction may apply To guest sessions, like
	// the inability of sending some commands or other nodes may want To block
	// messages originated by guest identities.
	AuthenticationSchemeGuest = AuthenticationScheme("guest")
	// AuthenticationSchemePlain Username and password authentication.
	AuthenticationSchemePlain = AuthenticationScheme("plain")
	// AuthenticationSchemeKey Key authentication.
	AuthenticationSchemeKey = AuthenticationScheme("key")
	// AuthenticationSchemeTransport Transport layer authentication.
	AuthenticationSchemeTransport = AuthenticationScheme("transport")
	// AuthenticationSchemeExternal Third-party authentication.
	AuthenticationSchemeExternal = AuthenticationScheme("external")
)

var authFactories = map[AuthenticationScheme]func() Authentication{
	AuthenticationSchemeGuest: func() Authentication {
		return &GuestAuthentication{}
	},
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

// Authentication defines a session authentications scheme container
type Authentication interface {
	GetAuthenticationScheme() AuthenticationScheme
}

// GuestAuthentication defines a guest authentication scheme.
type GuestAuthentication struct {
}

func (g *GuestAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeGuest
}

// PlainAuthentication defines a plain authentication scheme, that uses a password for authentication.
// Should be used only with encrypted sessions.
type PlainAuthentication struct {
	// Base64 representation of the password
	Password string `json:"password"`
}

func (a *PlainAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemePlain
}

func (a *PlainAuthentication) SetPasswordAsBase64(password string) {
	a.Password = base64.StdEncoding.EncodeToString([]byte(password))
}

// KeyAuthentication defines a plain authentication scheme, that uses a key for authentication.
// Should be used only with encrypted sessions.
type KeyAuthentication struct {
	// Base64 representation of the key
	Key string `json:"key"`
}

func (a *KeyAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeKey
}

func (a *KeyAuthentication) SetKeyAsBase64(key string) {
	a.Key = base64.StdEncoding.EncodeToString([]byte(key))
}

// TransportAuthentication defines a transport layer authentication scheme.
type TransportAuthentication struct {
}

func (a *TransportAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeTransport
}

// ExternalAuthentication defines an external authentication scheme, that uses third-party validation.
type ExternalAuthentication struct {
	// The authentication token on base64 representation.
	Token string `json:"token"`
	// The trusted token issuer.
	Issuer string `json:"issuer"`
}

func (a *ExternalAuthentication) GetAuthenticationScheme() AuthenticationScheme {
	return AuthenticationSchemeExternal
}
