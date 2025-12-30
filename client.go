package lime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Client allow the creation of a Lime connection with a server.
// The connection lifetime is handled automatically, with automatic reconnections in case of unrequested disconnections.
// There are methods for sending messages, notifications and command.
// It also allows the definition of handles for receiving these envelopes from the remote party.
type Client struct {
	config  *ClientConfig
	channel *ClientChannel
	mu      sync.RWMutex // mutex for setting the channel
	mux     *EnvelopeMux
	lock    chan struct{}      // lock is used as a mutex for channel lifetime handling operations
	cancel  context.CancelFunc // cancel stops the channel listener goroutine
	done    chan bool          // done is used by the listener goroutine to signal its end
}

// NewClient creates a new instance of the Client type.
func NewClient(config *ClientConfig, mux *EnvelopeMux) *Client {
	if config == nil {
		config = defaultClientConfig
	}
	if mux == nil || reflect.ValueOf(mux).IsNil() {
		panic("nil mux")
	}
	c := &Client{
		config: config,
		mux:    mux,
		lock:   make(chan struct{}, 1),
	}
	c.startListener()
	return c
}

// Establish forces the establishment of a session, in case of not being already established.
// It also awaits for any establishment operation that is in progress, returning only when it succeeds.
func (c *Client) Establish(ctx context.Context) error {
	_, err := c.getOrBuildChannel(ctx)
	return err
}

// Close stops the listener and finishes any established session with the server.
func (c *Client) Close() error {
	c.stopListener()

	if c.channel == nil {
		return nil
	}

	c.lock <- struct{}{}
	defer func() {
		<-c.lock
	}()

	if c.channel == nil {
		return nil
	}

	if c.channel.Established() {
		// Try to close the session gracefully
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
		defer cancelFunc()
		_, err := c.channel.FinishSession(ctx)
		c.channel = nil
		return err
	}

	err := c.channel.Close()
	c.channel = nil
	return err
}

// SendMessage asynchronously sends a Message to the server.
// The server may route the Message to another node, accordingly to the specified destination address.
// It may also send back one or more Notification envelopes, containing status about the Message.
func (c *Client) SendMessage(ctx context.Context, msg *Message) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendMessage(ctx, msg)
}

// SendNotification asynchronously sends a Notification to the server.
// The server may route the Notification to another node, accordingly to the specified destination address.
func (c *Client) SendNotification(ctx context.Context, not *Notification) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendNotification(ctx, not)
}

// SendRequestCommand asynchronously sends a RequestCommand to the server.
// The server may route the RequestCommand to another node, accordingly to the specified destination address.
// This method can be used for sending request and response commands, but in case of requests, it does not await for response.
// For receiving the response, the ProcessCommand method should be used.
func (c *Client) SendRequestCommand(ctx context.Context, cmd *RequestCommand) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendRequestCommand(ctx, cmd)
}

// ProcessCommand send a RequestCommand to the server and returns the corresponding ResponseCommand.
func (c *Client) ProcessCommand(ctx context.Context, cmd *RequestCommand) (*ResponseCommand, error) {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return nil, err
	}
	return channel.ProcessCommand(ctx, cmd)
}

func (c *Client) channelOK() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channel != nil && c.channel.Established()
}

func (c *Client) getOrBuildChannel(ctx context.Context) (*ClientChannel, error) {
	if c.channelOK() {
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.channel, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case c.lock <- struct{}{}:
		break
	}

	defer func() {
		<-c.lock
	}()

	if c.channelOK() {
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.channel, nil
	}

	count := 0.0

	for ctx.Err() == nil {
		if c.channel != nil {
			// don't care about the result,
			// calling close just to release resources.
			_ = c.channel.Close()
			c.mu.Lock()
			c.channel = nil
			c.mu.Unlock()
		}

		channel, err := c.buildChannel(ctx)
		if err == nil {
			c.mu.Lock()
			c.channel = channel
			c.mu.Unlock()
			return channel, nil
		}

		interval := time.Duration(count*count*100) * time.Millisecond
		log.Printf("build channel error on attempt %v, sleeping %v ms: %v", count, interval, err)

		// Use context-aware sleep to allow interruption
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("client: getOrBuildChannel: %w", ctx.Err())
		case <-time.After(interval):
		}
		count++
	}

	return nil, fmt.Errorf("client: getOrBuildChannel: %w", ctx.Err())
}

func (c *Client) startListener() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.done = make(chan bool)

	go func() {
		defer close(c.done)

		for ctx.Err() == nil {
			channel, err := c.getOrBuildChannel(ctx)
			if err != nil {
				log.Printf("client: listen: %v", err)
				continue
			}

			if err := c.mux.ListenClient(ctx, channel); err != nil {
				if errors.Is(err, context.Canceled) {
					continue
				}
				log.Printf("client: listen: %v", err)
			}
		}
	}()
}

func (c *Client) stopListener() {
	if c.cancel != nil {
		c.cancel()
		<-c.done
		c.cancel = nil
	}
}

func (c *Client) buildChannel(ctx context.Context) (*ClientChannel, error) {
	transport, err := c.config.NewTransport(ctx)
	if err != nil {
		return nil, fmt.Errorf("buildChannel: %w", err)
	}

	channel := NewClientChannel(transport, c.config.ChannelBufferSize)
	ses, err := channel.EstablishSession(
		ctx,
		c.config.CompSelector,
		c.config.EncryptSelector,
		c.config.Node.Identity,
		c.config.Authenticator,
		c.config.Node.Instance,
	)
	if err != nil {
		return nil, fmt.Errorf("buildChannel: %w", err)
	}

	if ses.State != SessionStateEstablished {
		return nil, fmt.Errorf("buildChannel: channel state is %v", ses.State)
	}

	return channel, nil
}

// ClientConfig defines the configurations for a Client instance.
type ClientConfig struct {
	// Node represents the address that the client should use in the session negotiation.
	// Note that the server may not use/accept the provided instance value, and it may be changed during the authentication.
	Node Node
	// The size of the internal envelope buffer used by the ClientChannel.
	// Greater values may improve the performance, but will also increase the process memory usage.
	ChannelBufferSize int
	// NewTransport represents the factory for Transport instances.
	NewTransport func(ctx context.Context) (Transport, error)
	// CompSelector is called during the session negotiation, for selecting the SessionCompression to be used.
	CompSelector CompressionSelector
	// EncryptSelector is called during the session negotiation, for selecting the SessionEncryption to be used.
	EncryptSelector EncryptionSelector
	// Authenticator is called during the session authentication and allows the client to provide its credentials
	// during the process.
	Authenticator Authenticator
}

var defaultClientConfig = NewClientConfig()

// NewClientConfig creates a new instance of ClientConfig with the default configuration values.
func NewClientConfig() *ClientConfig {
	instance, err := os.Hostname()
	if err != nil || instance == "" {
		instance = uuid.NewString()
	}

	return &ClientConfig{
		Node: Node{
			Identity: Identity{
				Name:   uuid.NewString(),
				Domain: "localhost",
			},
			Instance: instance,
		},
		ChannelBufferSize: runtime.NumCPU() * 32,
		NewTransport: func(ctx context.Context) (Transport, error) {
			return DialTcp(ctx, &net.TCPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: 55321,
			}, nil)
		},
		CompSelector: func(options []SessionCompression) SessionCompression {
			return options[0]
		},
		EncryptSelector: func(options []SessionEncryption) SessionEncryption {
			if contains(options, SessionEncryptionTLS) {
				return SessionEncryptionTLS
			}
			return options[0]
		},
		Authenticator: func(schemes []AuthenticationScheme, _ Authentication) Authentication {
			if contains(schemes, AuthenticationSchemeGuest) {
				return &GuestAuthentication{}
			}
			panic("Unsupported authentication scheme")
		},
	}
}

// ClientBuilder is a helper for building instances of Client.
// Avoid instantiating it directly, use the NewClientBuilder() function instead.
type ClientBuilder struct {
	config *ClientConfig
	mux    *EnvelopeMux
}

// NewClientBuilder creates a new ClientBuilder, which is a helper for building Client instances.
// It provides a fluent interface for convenience.
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{config: NewClientConfig(), mux: &EnvelopeMux{}}
}

// Name sets the client's node name.
func (b *ClientBuilder) Name(name string) *ClientBuilder {
	b.config.Node.Name = name
	return b
}

// Domain sets the client's node domain.
func (b *ClientBuilder) Domain(domain string) *ClientBuilder {
	b.config.Node.Domain = domain
	return b
}

// Instance sets the client's node instance.
func (b *ClientBuilder) Instance(instance string) *ClientBuilder {
	b.config.Node.Instance = instance
	return b
}

// MessageHandlerFunc allows the registration of a function for handling received messages that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ClientBuilder) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) *ClientBuilder {
	b.mux.MessageHandlerFunc(predicate, f)
	return b
}

// MessagesHandlerFunc allows the registration of a function for handling all received messages.
// This handler should be the last one to be registered, since it will capture all messages received by the client.
func (b *ClientBuilder) MessagesHandlerFunc(f MessageHandlerFunc) *ClientBuilder {
	b.mux.MessageHandlerFunc(func(msg *Message) bool { return true }, f)
	return b
}

// MessageHandler allows the registration of a MessageHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ClientBuilder) MessageHandler(handler MessageHandler) *ClientBuilder {
	b.mux.MessageHandler(handler)
	return b
}

// NotificationHandlerFunc allows the registration of a function for handling received notifications that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ClientBuilder) NotificationHandlerFunc(predicate NotificationPredicate, f NotificationHandlerFunc) *ClientBuilder {
	b.mux.NotificationHandlerFunc(predicate, f)
	return b
}

// NotificationsHandlerFunc allows the registration of a function for handling all received notifications.
// This handler should be the last one to be registered, since it will capture all notifications received by the client.
func (b *ClientBuilder) NotificationsHandlerFunc(f NotificationHandlerFunc) *ClientBuilder {
	b.mux.NotificationHandlerFunc(func(not *Notification) bool { return true }, f)
	return b
}

// NotificationHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ClientBuilder) NotificationHandler(handler NotificationHandler) *ClientBuilder {
	b.mux.NotificationHandler(handler)
	return b
}

// RequestCommandHandlerFunc allows the registration of a function for handling received commands that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ClientBuilder) RequestCommandHandlerFunc(predicate RequestCommandPredicate, f RequestCommandHandlerFunc) *ClientBuilder {
	b.mux.RequestCommandHandlerFunc(predicate, f)
	return b
}

// RequestCommandsHandlerFunc allows the registration of a function for handling all received commands.
// This handler should be the last one to be registered, since it will capture all commands received by the client.
func (b *ClientBuilder) RequestCommandsHandlerFunc(f RequestCommandHandlerFunc) *ClientBuilder {
	b.mux.RequestCommandHandlerFunc(func(cmd *RequestCommand) bool { return true }, f)
	return b
}

// RequestCommandHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ClientBuilder) RequestCommandHandler(handler RequestCommandHandler) *ClientBuilder {
	b.mux.RequestCommandHandler(handler)
	return b
}

// AutoReplyPings adds a RequestCommandHandler handler to automatically reply ping requests from the remote node.
func (b *ClientBuilder) AutoReplyPings() *ClientBuilder {
	return b.RequestCommandHandlerFunc(
		func(cmd *RequestCommand) bool {
			return cmd.Method == CommandMethodGet && cmd.URI.Path() == "/ping"
		},
		func(ctx context.Context, cmd *RequestCommand, s Sender) error {
			return s.SendResponseCommand(
				ctx,
				cmd.SuccessResponseWithResource(&Ping{}))
		})
}

// ResponseCommandHandlerFunc allows the registration of a function for handling received commands that matches
// the specified predicate. Note that the registration order matters, since the receiving process stops when
// the first predicate match occurs.
func (b *ClientBuilder) ResponseCommandHandlerFunc(predicate ResponseCommandPredicate, f ResponseCommandHandlerFunc) *ClientBuilder {
	b.mux.ResponseCommandHandlerFunc(predicate, f)
	return b
}

// ResponseCommandsHandlerFunc allows the registration of a function for handling all received commands.
// This handler should be the last one to be registered, since it will capture all commands received by the client.
func (b *ClientBuilder) ResponseCommandsHandlerFunc(f ResponseCommandHandlerFunc) *ClientBuilder {
	b.mux.ResponseCommandHandlerFunc(func(cmd *ResponseCommand) bool { return true }, f)
	return b
}

// ResponseCommandHandler allows the registration of a NotificationHandler.
// Note that the registration order matters, since the receiving process stops when the first predicate match occurs.
func (b *ClientBuilder) ResponseCommandHandler(handler ResponseCommandHandler) *ClientBuilder {
	b.mux.ResponseCommandHandler(handler)
	return b
}

// UseTCP adds a TCP listener to the server, allowing receiving connections from this transport.
func (b *ClientBuilder) UseTCP(addr net.Addr, config *TCPConfig) *ClientBuilder {
	b.config.NewTransport = func(ctx context.Context) (Transport, error) {
		return DialTcp(ctx, addr, config)
	}
	return b
}

// UseWebsocket adds a Websockets listener to the server, allowing receiving connections from this transport.
func (b *ClientBuilder) UseWebsocket(urlStr string, requestHeader http.Header, tls *tls.Config) *ClientBuilder {
	b.config.NewTransport = func(ctx context.Context) (Transport, error) {
		return DialWebsocket(ctx, urlStr, requestHeader, tls)
	}
	return b
}

// UseInProcess adds an in-process listener to the server, allowing receiving virtual connections from this transport.
func (b *ClientBuilder) UseInProcess(addr InProcessAddr, bufferSize int) *ClientBuilder {
	b.config.NewTransport = func(context.Context) (Transport, error) {
		return DialInProcess(addr, bufferSize)
	}
	return b
}

// GuestAuthentication enables the use of the guest authentication scheme during the session establishment with the server.
func (b *ClientBuilder) GuestAuthentication() *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &GuestAuthentication{}
	}
	return b
}

// TransportAuthentication enables the use of the transport authentication scheme during the session establishment with
// the server. Note that the transport that are being used to communicate with the server will be asked to present the
// credentials, and the form of passing the credentials may vary depending on the transport type. For instance, in
// TCP transport connections, the client certificate used during the mutual TLS negotiation is considered the
// credentials by the server.
func (b *ClientBuilder) TransportAuthentication() *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &TransportAuthentication{}
	}
	return b
}

// PlainAuthentication enables the use of the password authentication during the session establishment with the server.
func (b *ClientBuilder) PlainAuthentication(password string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		a := &PlainAuthentication{}
		a.SetPasswordAsBase64(password)
		return a
	}
	return b
}

// KeyAuthentication enables the use of the key authentication during the session establishment with the server.
func (b *ClientBuilder) KeyAuthentication(key string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		a := &KeyAuthentication{}
		a.SetKeyAsBase64(key)
		return a
	}
	return b
}

// ExternalAuthentication enables the use of the external authentication during the session establishment with the server.
func (b *ClientBuilder) ExternalAuthentication(token, issuer string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &ExternalAuthentication{Token: token, Issuer: issuer}
	}
	return b
}

// Compression sets the compression to be used in the session negotiation.
func (b *ClientBuilder) Compression(c SessionCompression) *ClientBuilder {
	b.config.CompSelector = func([]SessionCompression) SessionCompression {
		return c
	}
	return b
}

// Encryption sets the encryption to be used in the session negotiation.
func (b *ClientBuilder) Encryption(e SessionEncryption) *ClientBuilder {
	b.config.EncryptSelector = func([]SessionEncryption) SessionEncryption {
		return e
	}
	return b
}

// ChannelBufferSize is the size of the internal envelope buffer used by the ClientChannel.
// Greater values may improve the performance, but will also increase the process memory usage.
func (b *ClientBuilder) ChannelBufferSize(bufferSize int) *ClientBuilder {
	b.config.ChannelBufferSize = bufferSize
	return b
}

// Build creates a new instance of Client.
func (b *ClientBuilder) Build() *Client {
	return NewClient(b.config, b.mux)
}
