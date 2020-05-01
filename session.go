package main

import "fmt"

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
	// Authentication data, related to the selected schema.
	// Information like password sent by the client or roundtrip data sent by the server.
	Authentication Authentication `json:"authentication,omitempty"`
}

// Defines the supported session states
type SessionState string

const (
	// The session is new and doesn't exists an established context.
	// It is sent by a client node to start a session with a server.
	SessionNew = SessionState("new")
	// The server and the client are negotiating the session options,
	// like cryptography and compression.
	// The server sends to the client the options (if available) and
	// the client chooses the desired options. If there's no options
	// (for instance, if the connection is already encrypted or the
	// transport protocol doesn't support these options), the server
	// SHOULD skip the negotiation.
	SessionNegotiating = SessionState("negotiating")
	// The session is being authenticated. The server sends to
	// the client the available authentication schemes list and
	// the client must choose one and send the specific authentication
	// data. The authentication can occurs in multiple round trips,
	// according to the selected schema.
	SessionAuthenticating = SessionState("authenticating")
	// The session is active and it is possible to send and receive
	// messages and commands. The server sends this state
	// after the session was authenticated.
	SessionEstablished = SessionState("established")
	// The client node is requesting to the server to finish the session.
	SessionFinishing = SessionState("finishing")
	// The session was gracefully finished by the server.
	SessionFinished = SessionState("finished")
	// A problem occurred while the session was established, under
	// negotiation or authentication and it was closed by the server.
	// In this case, the property reason MUST be present to provide
	// more details about the problem.
	SessionFailed = SessionState("failed")
)

func (s SessionState) IsValid() error {
	switch s {
	case SessionNew, SessionNegotiating, SessionAuthenticating, SessionEstablished, SessionFinishing, SessionFinished, SessionFailed:
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
	// identity to the node. Some restriction may apply to guest sessions, like
	// the inability of sending some commands or other nodes may want to block
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
