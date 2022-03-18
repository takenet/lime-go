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

type Server struct {
	config        *ServerConfig
	mux           *EnvelopeMux
	listeners     []BoundListener
	mu            sync.Mutex
	transportChan chan Transport
	shutdown      context.CancelFunc
}

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
		SchemeOpts:        []AuthenticationScheme{AuthenticationSchemeGuest},
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

type ServerBuilder struct {
	config    *ServerConfig
	mux       *EnvelopeMux
	listeners []BoundListener

	plainAuth    PlainAuthenticator
	keyAuth      KeyAuthenticator
	externalAuth ExternalAuthenticator
}

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

func (b *ServerBuilder) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) *ServerBuilder {
	b.mux.MessageHandlerFunc(predicate, f)
	return b
}

func (b *ServerBuilder) MessagesHandlerFunc(f MessageHandlerFunc) *ServerBuilder {
	b.mux.MessageHandlerFunc(func(msg *Message) bool { return true }, f)
	return b
}

func (b *ServerBuilder) MessageHandler(handler MessageHandler) *ServerBuilder {
	b.mux.MessageHandler(handler)
	return b
}

func (b *ServerBuilder) NotificationHandlerFunc(predicate NotificationPredicate, f NotificationHandlerFunc) *ServerBuilder {
	b.mux.NotificationHandlerFunc(predicate, f)
	return b
}

func (b *ServerBuilder) NotificationsHandlerFunc(f NotificationHandlerFunc) *ServerBuilder {
	b.mux.NotificationHandlerFunc(func(not *Notification) bool { return true }, f)
	return b
}

func (b *ServerBuilder) NotificationHandler(handler NotificationHandler) *ServerBuilder {
	b.mux.NotificationHandler(handler)
	return b
}

func (b *ServerBuilder) CommandHandlerFunc(predicate CommandPredicate, f CommandHandlerFunc) *ServerBuilder {
	b.mux.CommandHandlerFunc(predicate, f)
	return b
}

func (b *ServerBuilder) CommandsHandlerFunc(f CommandHandlerFunc) *ServerBuilder {
	b.mux.CommandHandlerFunc(func(cmd *Command) bool { return true }, f)
	return b
}

func (b *ServerBuilder) CommandHandler(handler CommandHandler) *ServerBuilder {
	b.mux.CommandHandler(handler)
	return b
}

func (b *ServerBuilder) ListenTCP(addr *net.TCPAddr, config *TCPConfig) *ServerBuilder {
	listener := NewTCPTransportListener(config)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

func (b *ServerBuilder) ListenWebsocket(addr *net.TCPAddr, config *WebsocketConfig) *ServerBuilder {
	listener := NewWebsocketTransportListener(config)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

func (b *ServerBuilder) ListenInProcess(addr InProcessAddr) *ServerBuilder {
	listener := NewInProcessTransportListener(addr)
	b.listeners = append(b.listeners, NewBoundListener(listener, addr))
	return b
}

func (b *ServerBuilder) EnableGuestAuthentication() *ServerBuilder {
	if !contains(b.config.SchemeOpts, AuthenticationSchemeGuest) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeGuest)
	}
	return b
}

func (b *ServerBuilder) EnableTransportAuthentication() *ServerBuilder {
	if !contains(b.config.SchemeOpts, AuthenticationSchemeTransport) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeTransport)
	}
	return b
}

type PlainAuthenticator func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error)

func (b *ServerBuilder) EnablePlainAuthentication(a PlainAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}

	if !contains(b.config.SchemeOpts, AuthenticationSchemePlain) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemePlain)
	}
	return b
}

type KeyAuthenticator func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error)

func (b *ServerBuilder) EnableKeyAuthentication(a KeyAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}

	if !contains(b.config.SchemeOpts, AuthenticationSchemeKey) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeKey)
	}
	return b
}

type ExternalAuthenticator func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error)

func (b *ServerBuilder) EnableExternalAuthentication(a ExternalAuthenticator) *ServerBuilder {
	if a == nil {
		panic("nil authenticator")
	}

	if !contains(b.config.SchemeOpts, AuthenticationSchemeExternal) {
		b.config.SchemeOpts = append(b.config.SchemeOpts, AuthenticationSchemeExternal)
	}
	return b
}

func (b *ServerBuilder) ChannelBufferSize(bufferSize int) *ServerBuilder {
	b.config.ChannelBufferSize = bufferSize
	return b
}

func (b *ServerBuilder) Register(register func(ctx context.Context, candidate Node, c *ServerChannel) (Node, error)) *ServerBuilder {
	b.config.Register = register
	return b
}

func (b *ServerBuilder) Established(established func(sessionID string, c *ServerChannel)) *ServerBuilder {
	b.config.Established = established
	return b
}

func (b *ServerBuilder) Finished(finished func(sessionID string)) *ServerBuilder {
	b.config.Finished = finished
	return b
}

func (b *ServerBuilder) Build() *Server {
	b.config.Authenticate = buildAuthenticate(b.plainAuth, b.keyAuth, b.externalAuth)
	return NewServer(b.config, b.mux, b.listeners...)
}

func buildAuthenticate(plainAuth PlainAuthenticator, keyAuth KeyAuthenticator, externalAuth ExternalAuthenticator) func(ctx context.Context, identity Identity, authentication Authentication) (*AuthenticationResult, error) {
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
			return plainAuth(ctx, identity, a.Password)
		case *KeyAuthentication:
			if keyAuth == nil {
				return nil, errors.New("key authenticator is nil")
			}
			return keyAuth(ctx, identity, a.Key)
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
