package lime

import (
	"net"
	"reflect"
)

type Server struct {
	config    *ServerConfig
	mux       *EnvelopeMux
	listeners []TransportListener
}

type ServerConfig struct {
	Node         Node                   // Node represents the server's address.
	CompOpts     []SessionCompression   // CompOpts defines the compression options to be used in the session negotiation.
	EncryptOpts  []SessionEncryption    // EncryptOpts defines the encryption options to be used in the session negotiation.
	SchemeOpts   []AuthenticationScheme // SchemeOpts defines the authentication schemes that should be presented to the clients during session establishment.
	Authenticate func(Identity, Authentication) (*AuthenticationResult, error)
	Register     func(Node, *ServerChannel) (Node, error)
}

func NewServer(config *ServerConfig, mux *EnvelopeMux, listeners ...TransportListener) *Server {
	if mux == nil || reflect.ValueOf(mux).IsNil() {
		panic("nil mux")
	}
	if len(listeners) == 0 {
		panic("empty listeners")
	}
	return &Server{config: config, mux: mux, listeners: listeners}
}

func (s *Server) ListenAndServe() error {
	return nil
}

type ServerBuilder struct {
	node      Node
	mux       *EnvelopeMux
	addrs     []net.Addr
	listeners []TransportListener
}

func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{node: Node{
		Identity: Identity{
			Name:   "postmaster",
			Domain: "localhost",
		},
		Instance: "",
	}, mux: &EnvelopeMux{}}
}

// Name sets the server's node name.
func (b *ServerBuilder) Name(name string) *ServerBuilder {
	b.node.Name = name
	return b
}

// Domain sets the server's node domain.
func (b *ServerBuilder) Domain(domain string) *ServerBuilder {
	b.node.Domain = domain
	return b
}

// Instance sets the server's node instance.
func (b *ServerBuilder) Instance(instance string) *ServerBuilder {
	b.node.Instance = instance
	return b
}

func (b *ServerBuilder) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) *ServerBuilder {
	b.mux.MessageHandlerFunc(predicate, f)
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

func (b *ServerBuilder) NotificationHandler(handler NotificationHandler) *ServerBuilder {
	b.mux.NotificationHandler(handler)
	return b
}

func (b *ServerBuilder) CommandHandlerFunc(predicate CommandPredicate, f CommandHandlerFunc) *ServerBuilder {
	b.mux.CommandHandlerFunc(predicate, f)
	return b
}

func (b *ServerBuilder) CommandHandler(handler CommandHandler) *ServerBuilder {
	b.mux.CommandHandler(handler)
	return b
}

func (b *ServerBuilder) ListenTCP(addr net.TCPAddr, config *TCPConfig) *ServerBuilder {
	listener := NewTCPTransportListener(config)
	b.listeners = append(b.listeners, listener)
	b.addrs = append(b.addrs, &addr)
	return b
}

func (b *ServerBuilder) ListenWebsocket(addr net.TCPAddr, config *WebsocketConfig) *ServerBuilder {
	listener := NewWebsocketTransportListener(config)
	b.listeners = append(b.listeners, listener)
	b.addrs = append(b.addrs, &addr)
	return b
}

func (b *ServerBuilder) ListenInProcess(addr InProcessAddr) *ServerBuilder {
	listener := NewInProcessTransportListener(addr)
	b.listeners = append(b.listeners, listener)
	b.addrs = append(b.addrs, &addr)
	return b
}

func (b *ServerBuilder) Build() *Server {
	config := &ServerConfig{
		Node:         b.node,
		CompOpts:     nil,
		EncryptOpts:  nil,
		SchemeOpts:   nil,
		Authenticate: nil,
		Register:     nil,
	}
	return NewServer(config, b.mux, b.listeners...)
}
