package lime

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Allows the configuration and establishment of the communication channel between nodes.
type Session struct {
	EnvelopeBase
	// Informs or changes the state of a session.
	// Only the server can change the session state, but the client can request
	// the state transition.
	State SessionState
	// Encryption options provided by the server during the session negotiation.
	EncryptionOptions []SessionEncryption
	// The encryption option selected for the session.
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
	Reason Reason
}

func (s *Session) SetAuthentication(a Authentication) {
	s.Authentication = a
	s.Scheme = a.GetAuthenticationScheme()
}

// Wrapper for custom marshalling
type SessionWrapper struct {
	EnvelopeBaseWrapper
	State              SessionState           `json:"state"`
	EncryptionOptions  []SessionEncryption    `json:"encryptionOptions,omitempty"`
	Encryption         SessionEncryption      `json:"encryption,omitempty"`
	CompressionOptions []SessionCompression   `json:"compressionOptions,omitempty"`
	Compression        SessionCompression     `json:"compression,omitempty"`
	SchemeOptions      []AuthenticationScheme `json:"schemeOptions,omitempty"`
	Scheme             AuthenticationScheme   `json:"scheme,omitempty"`
	Authentication     *json.RawMessage       `json:"authentication,omitempty"`
	Reason             *Reason                `json:"reason,omitempty"`
}

func (s Session) MarshalJSON() ([]byte, error) {
	sw, err := s.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(sw)
}

func (s *Session) UnmarshalJSON(b []byte) error {
	cw := SessionWrapper{}
	err := json.Unmarshal(b, &cw)
	if err != nil {
		return err
	}

	command := Session{}
	err = command.populate(&cw)
	if err != nil {
		return err
	}

	*s = command
	return nil
}

func (s *Session) toWrapper() (SessionWrapper, error) {
	ew, err := s.EnvelopeBase.toWrapper()
	if err != nil {
		return SessionWrapper{}, err
	}

	sw := SessionWrapper{
		EnvelopeBaseWrapper: ew,
	}

	if s.Authentication != nil {
		b, err := json.Marshal(s.Authentication)
		if err != nil {
			return SessionWrapper{}, err
		}
		a := json.RawMessage(b)
		sw.Authentication = &a
		sw.Scheme = s.Scheme
	}

	sw.State = s.State
	sw.EncryptionOptions = s.EncryptionOptions
	sw.Encryption = s.Encryption
	sw.CompressionOptions = s.CompressionOptions
	sw.Compression = s.Compression
	sw.SchemeOptions = s.SchemeOptions
	sw.Scheme = s.Scheme

	if s.Reason != (Reason{}) {
		sw.Reason = &s.Reason
	}

	return sw, nil
}

func (s *Session) populate(sw *SessionWrapper) error {
	err := s.EnvelopeBase.populate(&sw.EnvelopeBaseWrapper)
	if err != nil {
		return err
	}

	// Create the auth type instance and unmarshal the json to it
	if sw.Authentication != nil {
		if sw.Scheme == "" {
			return errors.New("session scheme is required when authentication is present")
		}

		factory, ok := authFactories[sw.Scheme]
		if !ok {
			return fmt.Errorf(`unknown authentication scheme '%v'`, sw.Scheme)
		}
		a := factory()
		err := json.Unmarshal(*sw.Authentication, &a)
		if err != nil {
			return err
		}

		s.Authentication = a
		s.Scheme = sw.Scheme
	}

	s.State = sw.State
	s.EncryptionOptions = sw.EncryptionOptions
	s.Encryption = sw.Encryption
	s.CompressionOptions = sw.CompressionOptions
	s.Compression = sw.Compression
	s.SchemeOptions = sw.SchemeOptions
	s.Scheme = sw.Scheme

	if sw.Reason != nil {
		s.Reason = *sw.Reason
	}

	return nil
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

// Defines the valid session compression values.
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
