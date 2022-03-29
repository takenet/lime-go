package lime

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go.uber.org/multierr"
	"golang.org/x/sync/errgroup"
	"log"
	"net"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"
)

// Server allows the receiving of Lime connections through multiple transport listeners, like TCP and Websockets.
// It handles the session negotiation and authentication through the handlers defined in ServerConfig.
// Finally, it allows the definition of handles for receiving envelopes from the clients.
type Server struct {
	config        *ServerConfig
	mux           *EnvelopeMux
	listeners     []BoundListener
	mu            sync.Mutex
	transportChan chan Transport
	shutdown      context.CancelFunc
}

// NewServer creates a new instance of the Server type.
func NewServer(config *ServerConfig, mux *EnvelopeMux, listeners ...BoundListener) *Server {
	if config == nil {
		config = defaultServerConfig
	}
	if mux == nil || reflect.ValueOf(mux).IsNil() {
		panic("nil mux")
	}
	if len(listeners) == 0 {
		panic("empty listeners")
	}
	return &Server{
		config:        config,
		mux:           mux,
		listeners:     listeners,
		transportChan: make(chan Transport, config.Backlog),
	}
}

// ListenAndServe starts listening for new connections in the registered transport listeners.
// This is a blocking call which always returns a non nil error.
// In case of a graceful closing, the returned error is ErrServerClosed.
func (srv *Server) ListenAndServe() error {
	if srv.shutdown != nil {
		return errors.New("server already listening")
	}

	ctx, cancel := context.WithCancel(context.Background())
	srv.shutdown = cancel

	if len(srv.listeners) == 0 {
		return errors.New("no listeners found")
	}

	eg, ctx := errgroup.WithContext(ctx)

	for _, l := range srv.listeners {
		if err := l.Listener.Listen(ctx, l.Addr); err != nil {
			return fmt.Errorf("listen error: %w", err)
		}

		listener := l

		eg.Go(func() error {
			return acceptTransports(ctx, listener.Listener, srv.transportChan)
		})
	}

	eg.Go(func() error {
		srv.consumeTransports(ctx)
		return nil
	})

	err := eg.Wait()

	if errors.Is(err, ctx.Err()) {
		return ErrServerClosed
	}
	return err
}

func acceptTransports(ctx context.Context, listener TransportListener, c chan<- Transport) error {
	for {
		transport, err := listener.Accept(ctx)
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case c <- transport:
		}
	}
}

func (srv *Server) consumeTransports(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-srv.transportChan:
			c := NewServerChannel(t, srv.config.ChannelBufferSize, srv.config.Node, uuid.NewString())
			go func() {
				srv.handleChannel(ctx, c)
			}()
		}
	}
}

func (srv *Server) handleChannel(ctx context.Context, c *ServerChannel) {
	err := c.EstablishSession(
		ctx,
		srv.config.CompOpts,
		srv.config.EncryptOpts,
		srv.config.SchemeOpts,
		srv.config.Authenticate,
		srv.config.Register,
	)

	if err != nil {
		log.Printf("server: establish: %v\n", err)
		return
	}

	established := srv.config.Established
	if established != nil {
		established(c.sessionID, c)
	}

	defer func() {
		if c.Established() {
			// Do not use the shared context since it could be canceled
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = c.FinishSession(ctx)
		}

		finished := srv.config.Finished
		if finished != nil {
			finished(c.sessionID)
		}
	}()

	if err = srv.mux.ListenServer(ctx, c); err != nil {
		log.Printf("server: listen: %v\n", err)
		return
	}
}

// Close stops the server by closing the transport listeners and all active sessions.
func (srv *Server) Close() error {
	srv.mu.Lock()
	defer srv.mu.Unlock()

	if srv.shutdown == nil {
		return errors.New("server not listening")
	}

	srv.shutdown()
	srv.shutdown = nil

	var errs []error

	for _, listener := range srv.listeners {
		if err := listener.Listener.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	close(srv.transportChan)
	return multierr.Combine(errs...)
}

// ServerConfig define the configurations for a Server instance.
type ServerConfig struct {
	Node              Node                   // Node represents the server's address.
	CompOpts          []SessionCompression   // CompOpts defines the compression options to be used in the session negotiation.
	EncryptOpts       []SessionEncryption    // EncryptOpts defines the encryption options to be used in the session negotiation.
	SchemeOpts        []AuthenticationScheme // SchemeOpts defines the authentication schemes that should be presented to the clients during session establishment.
	Backlog           int                    // Backlog defines the size of the listener's pending connections queue.
	ChannelBufferSize int                    // ChannelBufferSize determines the internal envelope buffer size for the channels.

	// Authenticate is called for authenticating a client session.
	// It should return an AuthenticationResult instance with DomainRole different of DomainRoleUnknown for a successful authentication.
	Authenticate func(ctx context.Context, identity Identity, a Authentication) (*AuthenticationResult, error)
	// Register is called for the client Node address registration.
	// It receives a candidate node from the client and should return the effective node address that will be assigned
	// to the session.
	Register func(ctx context.Context, candidate Node, c *ServerChannel) (Node, error)
	// Established is called when a session with a node is established.
	Established func(sessionID string, c *ServerChannel)
	// Finished is called when an established session with a node is finished.
	Finished func(sessionID string)
}

var defaultServerConfig = NewServerConfig()

// NewServerConfig creates a new instance of ServerConfig with the default configuration values.
func NewServerConfig() *ServerConfig {
	instance, err := os.Hostname()
	if err != nil || instance == "" {
		instance = uuid.NewString()
	}
	return &ServerConfig{
		Node: Node{
			Identity: Identity{
				Name:   "postmaster",
				Domain: "localhost",
			},
			Instance: instance,
		},
		CompOpts:          []SessionCompression{SessionCompressionNone},
		EncryptOpts:       []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS},
		SchemeOpts:        []AuthenticationScheme{AuthenticationSchemeTransport},
		Backlog:           runtime.NumCPU() * 8,
		ChannelBufferSize: runtime.NumCPU() * 32,
		Authenticate: func(ctx context.Context, identity Identity, authentication Authentication) (*AuthenticationResult, error) {
			return MemberAuthenticationResult(), nil
		},
		Register: func(ctx context.Context, node Node, serverChannel *ServerChannel) (Node, error) {
			return Node{
				Identity: Identity{
					Name:   uuid.New().String(),
					Domain: serverChannel.localNode.Domain,
				},
				Instance: serverChannel.localNode.Instance}, nil
		},
	}
}

// ServerBuilder is a helper for building instances of Server.
// Avoid instantiating it directly, use the NewServerBuilder() function instead.
type ServerBuilder struct {
	config    *ServerConfig
	mux       *EnvelopeMux
	listeners []BoundListener

	plainAuth    PlainAuthenticator
	keyAuth      KeyAuthenticator
	externalAuth ExternalAuthenticator
}

// NewServerBuilder creates a new ServerBuilder, which is a helper for building Server instances.
// It provides a fluent interface for convenience.
func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{config: NewServerConfig(), mux: &EnvelopeMux{}}
}

// Name sets the server's node name.
func (b *ServerBuilder) Name(name string) *ServerBuilder {
	b.config.Node.Name = name
	return b
}

// Domain sets the server's node domain.
func (b *ServerBuilder) Domain(domain string) *ServerBuilder {
	b.config.Node.Domain = domain
	return b
}

// Instance sets the server's node instance.
func (b *ServerBuilder) Instance(instance string) *ServerBuilder {
	b.config.Node.Instance = instance
	return b
}

// MessageHandlerFunc allows the registration of a function for handling received messages that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ServerBuilder) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) *ServerBuilder {
	b.mux.MessageHandlerFunc(predicate, f)
	return b
}

// MessagesHandlerFunc allows the registration of a function for handling all received messages.
// This handler should be the last one to be registered, since it will capture all messages received by the client.
func (b *ServerBuilder) MessagesHandlerFunc(f MessageHandlerFunc) *ServerBuilder {
	b.mux.MessageHandlerFunc(func(msg *Message) bool { return true }, f)
	return b
}

// MessageHandler allows the registration of a MessageHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ServerBuilder) MessageHandler(handler MessageHandler) *ServerBuilder {
	b.mux.MessageHandler(handler)
	return b
}

// NotificationHandlerFunc allows the registration of a function for handling received notifications that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ServerBuilder) NotificationHandlerFunc(predicate NotificationPredicate, f NotificationHandlerFunc) *ServerBuilder {
	b.mux.NotificationHandlerFunc(predicate, f)
	return b
}

// NotificationsHandlerFunc allows the registration of a function for handling all received notifications.
// This handler should be the last one to be registered, since it will capture all notifications received by the client.
func (b *ServerBuilder) NotificationsHandlerFunc(f NotificationHandlerFunc) *ServerBuilder {
	b.mux.NotificationHandlerFunc(func(not *Notification) bool { return true }, f)
	return b
}

// NotificationHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ServerBuilder) NotificationHandler(handler NotificationHandler) *ServerBuilder {
	b.mux.NotificationHandler(handler)
	return b
}

// RequestCommandHandlerFunc allows the registration of a function for handling received commands that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ServerBuilder) RequestCommandHandlerFunc(predicate RequestCommandPredicate, f RequestCommandHandlerFunc) *ServerBuilder {
	b.mux.RequestCommandHandlerFunc(predicate, f)
	return b
}

// RequestCommandsHandlerFunc allows the registration of a function for handling all received commands.
// This handler should be the last one to be registered, since it will capture all commands received by the client.
func (b *ServerBuilder) RequestCommandsHandlerFunc(f RequestCommandHandlerFunc) *ServerBuilder {
	b.mux.RequestCommandHandlerFunc(func(cmd *RequestCommand) bool { return true }, f)
	return b
}

// RequestCommandHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ServerBuilder) RequestCommandHandler(handler RequestCommandHandler) *ServerBuilder {
	b.mux.RequestCommandHandler(handler)
	return b
}

// ResponseCommandHandlerFunc allows the registration of a function for handling received commands that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ServerBuilder) ResponseCommandHandlerFunc(predicate ResponseCommandPredicate, f ResponseCommandHandlerFunc) *ServerBuilder {
	b.mux.ResponseCommandHandlerFunc(predicate, f)
	return b
}

// ResponseCommandsHandlerFunc allows the registration of a function for handling all received commands.
// This handler should be the last one to be registered, since it will capture all commands received by the client.
func (b *ServerBuilder) ResponseCommandsHandlerFunc(f ResponseCommandHandlerFunc) *ServerBuilder {
	b.mux.ResponseCommandHandlerFunc(func(cmd *ResponseCommand) bool { return true }, f)
	return b
}

// ResponseCommandHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ServerBuilder) ResponseCommandHandler(handler ResponseCommandHandler) *ServerBuilder {
	b.mux.ResponseCommandHandler(handler)
	return b
}

// ListenTCP adds a new TCP transport listener with the specified configuration.
// This method can be called multiple times.
func (b *ServerBuilder) ListenTCP(addr *net.TCPAddr, config *TCPConfig) *ServerBuilder {
	listener := NewTCPTransportListener(config)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

// ListenWebsocket adds a new Websocket transport listener with the specified configuration.
// This method can be called multiple times.
func (b *ServerBuilder) ListenWebsocket(addr *net.TCPAddr, config *WebsocketConfig) *ServerBuilder {
	listener := NewWebsocketTransportListener(config)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

// ListenInProcess adds a new in-process transport listener with the specified configuration.
// This method can be called multiple times.
func (b *ServerBuilder) ListenInProcess(addr InProcessAddr) *ServerBuilder {
	listener := NewInProcessTransportListener(addr)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

// CompressionOptions defines the compression options to be used in the session negotiation.
func (b *ServerBuilder) CompressionOptions(compOpts ...SessionCompression) *ServerBuilder {
	if len(compOpts) == 0 {
		panic("empty compOpts")
	}
	b.config.CompOpts = compOpts
	return b
}

// EncryptionOptions defines the encryption options to be used in the session negotiation.
func (b *ServerBuilder) EncryptionOptions(encryptOpts ...SessionEncryption) *ServerBuilder {
	if len(encryptOpts) == 0 {
		panic("empty encryptOpts")
	}
	b.config.EncryptOpts = encryptOpts
	return b
}

// EnableGuestAuthentication enables the use of guest authentication scheme during the authentication of the
// client sessions.
// The guest authentication scheme do not require any credentials from the clients.
func (b *ServerBuilder) EnableGuestAuthentication() *ServerBuilder {
	if !contains(b.config.SchemeOpts, AuthenticationSchemeGuest) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeGuest)
	}
	return b
}

// EnableTransportAuthentication enables the use of transport authentication scheme during the authentication of the
// client sessions.
// The transport authentication will delegate the authentication to the session transport and  the form of passing the
// credentials may vary depending on the transport type.
// For instance, in TCP transport connections, the client certificate used during the mutual TLS negotiation is
// considered the credentials by the server.
func (b *ServerBuilder) EnableTransportAuthentication() *ServerBuilder {
	if !contains(b.config.SchemeOpts, AuthenticationSchemeTransport) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeTransport)
	}
	return b
}

// PlainAuthenticator defines a function for authenticating an identity session using a password.
type PlainAuthenticator func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error)

// EnablePlainAuthentication enables the use of plain authentication scheme during the authentication of the
// client sessions. The provided PlainAuthentication function is called for authenticating any session with this scheme.
func (b *ServerBuilder) EnablePlainAuthentication(a PlainAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}
	b.plainAuth = a
	if !contains(b.config.SchemeOpts, AuthenticationSchemePlain) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemePlain)
	}

	return b
}

// KeyAuthenticator defines a function for authenticating an identity session using a key.
type KeyAuthenticator func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error)

// EnableKeyAuthentication enables the use of key authentication scheme during the authentication of the
// client sessions. The provided KeyAuthenticator function is called for authenticating any session with this scheme.
func (b *ServerBuilder) EnableKeyAuthentication(a KeyAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}
	b.keyAuth = a
	if !contains(b.config.SchemeOpts, AuthenticationSchemeKey) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeKey)
	}
	return b
}

// ExternalAuthenticator defines a function for authenticating an identity session using tokens emitted by an issuer.
type ExternalAuthenticator func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error)

// EnableExternalAuthentication enables the use of key authentication scheme during the authentication of the
// client sessions. The provided ExternalAuthenticator function is called for authenticating any session with this scheme.
func (b *ServerBuilder) EnableExternalAuthentication(a ExternalAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}
	b.externalAuth = a
	if !contains(b.config.SchemeOpts, AuthenticationSchemeExternal) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeExternal)
	}
	return b
}

// ChannelBufferSize determines the internal envelope buffer size for the channels.
func (b *ServerBuilder) ChannelBufferSize(bufferSize int) *ServerBuilder {
	b.config.ChannelBufferSize = bufferSize
	return b
}

// Register is called for the client Node address registration.
// It receives a candidate node from the client and should return the effective node address that will be assigned
// to the session.
func (b *ServerBuilder) Register(register func(ctx context.Context, candidate Node, c *ServerChannel) (Node, error)) *ServerBuilder {
	b.config.Register = register
	return b
}

// Established is called when a session with a node is established.
func (b *ServerBuilder) Established(established func(sessionID string, c *ServerChannel)) *ServerBuilder {
	b.config.Established = established
	return b
}

// Finished is called when an established session with a node is finished.
func (b *ServerBuilder) Finished(finished func(sessionID string)) *ServerBuilder {
	b.config.Finished = finished
	return b
}

// Build creates a new instance of Server.
func (b *ServerBuilder) Build() *Server {
	b.config.Authenticate = buildAuthenticate(b.plainAuth, b.keyAuth, b.externalAuth)
	return NewServer(b.config, b.mux, b.listeners...)
}

func buildAuthenticate(plainAuth PlainAuthenticator, keyAuth KeyAuthenticator, externalAuth ExternalAuthenticator) func(
	ctx context.Context,
	identity Identity,
	authentication Authentication,
) (*AuthenticationResult, error) {
	return func(ctx context.Context, identity Identity, authentication Authentication) (*AuthenticationResult, error) {
		switch a := authentication.(type) {
		case *GuestAuthentication:
			return MemberAuthenticationResult(), nil
		case *TransportAuthentication:
			return nil, errors.New("transport auth not implemented yet")
		case *PlainAuthentication:
			if plainAuth == nil {
				return nil, errors.New("plain authenticator is nil")
			}
			pwd, err := a.GetPasswordFromBase64()
			if err != nil {
				return nil, fmt.Errorf("plain authenticator: %w", err)
			}
			return plainAuth(ctx, identity, pwd)
		case *KeyAuthentication:
			if keyAuth == nil {
				return nil, errors.New("key authenticator is nil")
			}
			key, err := a.GetKeyFromBase64()
			if err != nil {
				return nil, fmt.Errorf("key authenticator: %w", err)
			}
			return keyAuth(ctx, identity, key)
		case *ExternalAuthentication:
			if externalAuth == nil {
				return nil, errors.New("external authenticator is nil")
			}
			return externalAuth(ctx, identity, a.Token, a.Issuer)
		}

		return nil, errors.New("unknown authentication scheme")
	}
}

// BoundListener represents a pair of a TransportListener and a net.Addr values.
type BoundListener struct {
	Listener TransportListener
	Addr     net.Addr
}

func NewBoundListener(listener TransportListener, addr net.Addr) BoundListener {
	if listener == nil || reflect.ValueOf(listener).IsNil() {
		panic("nil Listener")
	}
	if addr == nil || reflect.ValueOf(addr).IsZero() {
		panic("zero addr value")
	}
	return BoundListener{
		Listener: listener,
		Addr:     addr,
	}
}

// ErrServerClosed is returned by the Server's ListenAndServe,
// method after a call to Close.
var ErrServerClosed = errors.New("lime: Server closed")
