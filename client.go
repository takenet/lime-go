package lime

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/google/uuid"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"sync"
	"time"
)

type Client struct {
	config  *ClientConfig
	channel *ClientChannel
	mux     *EnvelopeMux
	mu      sync.Mutex
}

func NewClient(config *ClientConfig, mux *EnvelopeMux) *Client {
	if config == nil {
		config = defaultClientConfig
	}
	if mux == nil || reflect.ValueOf(mux).IsNil() {
		panic("nil mux")
	}
	return &Client{config: config, mux: mux}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	err := c.channel.transport.Close()
	c.channel = nil
	return err
}

func (c *Client) SendMessage(ctx context.Context, msg *Message) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendMessage(ctx, msg)
}

func (c *Client) SendNotification(ctx context.Context, not *Notification) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendNotification(ctx, not)
}

func (c *Client) SendCommand(ctx context.Context, cmd *Command) error {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return err
	}
	return channel.SendCommand(ctx, cmd)
}

func (c *Client) ProcessCommand(ctx context.Context, cmd *Command) (*Command, error) {
	channel, err := c.getOrBuildChannel(ctx)
	if err != nil {
		return nil, err
	}
	return channel.ProcessCommand(ctx, cmd)
}

func (c *Client) channelOK() bool {
	return c.channel != nil && c.channel.Established()
}

func (c *Client) getOrBuildChannel(ctx context.Context) (*ClientChannel, error) {
	if c.channelOK() {
		return c.channel, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.channelOK() {
		return c.channel, nil
	}

	count := 0.0

	for ctx.Err() == nil {
		channel, err := c.buildChannel(ctx)
		if err == nil {
			if c.channel != nil {
				// don't care about the result,
				// calling close just to release resources.
				_ = c.channel.Close()
			}
			c.channel = channel
			return channel, nil
		}

		interval := time.Duration(math.Pow(count, 2) * 100)
		log.Printf("build channel error on attempt %v, sleeping %v ms: %v", count, interval, err)
		time.Sleep(interval * time.Millisecond)
		count++
	}

	return nil, fmt.Errorf("getOrBuildChannel: %w", ctx.Err())
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

type ClientConfig struct {
	Node              Node
	ChannelBufferSize int
	NewTransport      func(ctx context.Context) (Transport, error)
	CompSelector      CompressionSelector
	EncryptSelector   EncryptionSelector
	Authenticator     Authenticator
}

var defaultClientConfig = NewClientConfig()

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
		CompSelector: func(compressions []SessionCompression) SessionCompression {
			return compressions[0]
		},
		EncryptSelector: func(encryptions []SessionEncryption) SessionEncryption {
			if contains(encryptions, SessionEncryptionTLS) {
				return SessionEncryptionTLS
			}
			return encryptions[0]
		},
		Authenticator: func(schemes []AuthenticationScheme, authentication Authentication) Authentication {
			if contains(schemes, AuthenticationSchemeGuest) {
				return &GuestAuthentication{}
			}
			panic("Unsupported authentication scheme")
		},
	}
}

type ClientBuilder struct {
	config *ClientConfig
	mux    *EnvelopeMux
}

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

func (b *ClientBuilder) MessageHandlerFunc(predicate MessagePredicate, f MessageHandlerFunc) *ClientBuilder {
	b.mux.MessageHandlerFunc(predicate, f)
	return b
}

func (b *ClientBuilder) MessagesHandlerFunc(f MessageHandlerFunc) *ClientBuilder {
	b.mux.MessageHandlerFunc(func(msg *Message) bool { return true }, f)
	return b
}

func (b *ClientBuilder) MessageHandler(handler MessageHandler) *ClientBuilder {
	b.mux.MessageHandler(handler)
	return b
}

func (b *ClientBuilder) NotificationHandlerFunc(predicate NotificationPredicate, f NotificationHandlerFunc) *ClientBuilder {
	b.mux.NotificationHandlerFunc(predicate, f)
	return b
}

func (b *ClientBuilder) NotificationsHandlerFunc(f NotificationHandlerFunc) *ClientBuilder {
	b.mux.NotificationHandlerFunc(func(not *Notification) bool { return true }, f)
	return b
}

func (b *ClientBuilder) NotificationHandler(handler NotificationHandler) *ClientBuilder {
	b.mux.NotificationHandler(handler)
	return b
}

func (b *ClientBuilder) CommandHandlerFunc(predicate CommandPredicate, f CommandHandlerFunc) *ClientBuilder {
	b.mux.CommandHandlerFunc(predicate, f)
	return b
}

func (b *ClientBuilder) CommandsHandlerFunc(f CommandHandlerFunc) *ClientBuilder {
	b.mux.CommandHandlerFunc(func(cmd *Command) bool { return true }, f)
	return b
}

func (b *ClientBuilder) CommandHandler(handler CommandHandler) *ClientBuilder {
	b.mux.CommandHandler(handler)
	return b
}

func (b *ClientBuilder) UseTCP(addr net.Addr, config *TCPConfig) *ClientBuilder {
	b.config.NewTransport = func(ctx context.Context) (Transport, error) {
		return DialTcp(ctx, addr, config)
	}
	return b
}

func (b *ClientBuilder) UseWebsocket(urlStr string, requestHeader http.Header, tls *tls.Config) *ClientBuilder {
	b.config.NewTransport = func(ctx context.Context) (Transport, error) {
		return DialWebsocket(ctx, urlStr, requestHeader, tls)
	}
	return b
}

func (b *ClientBuilder) UseInProcess(addr InProcessAddr, bufferSize int) *ClientBuilder {
	b.config.NewTransport = func(context.Context) (Transport, error) {
		return DialInProcess(addr, bufferSize)
	}
	return b
}

func (b *ClientBuilder) GuestAuthentication() *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &GuestAuthentication{}
	}
	return b
}

func (b *ClientBuilder) TransportAuthentication() *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &TransportAuthentication{}
	}
	return b
}

func (b *ClientBuilder) PlainAuthentication(password string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		a := &PlainAuthentication{}
		a.SetPasswordAsBase64(password)
		return a
	}
	return b
}

func (b *ClientBuilder) KeyAuthentication(key string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		a := &KeyAuthentication{}
		a.SetKeyAsBase64(key)
		return a
	}
	return b
}

func (b *ClientBuilder) ExternalAuthentication(token, issuer string) *ClientBuilder {
	b.config.Authenticator = func([]AuthenticationScheme, Authentication) Authentication {
		return &ExternalAuthentication{Token: token, Issuer: issuer}
	}
	return b
}

func (b *ClientBuilder) Compression(c SessionCompression) *ClientBuilder {
	b.config.CompSelector = func([]SessionCompression) SessionCompression {
		return c
	}
	return b
}

func (b *ClientBuilder) Encryption(e SessionEncryption) *ClientBuilder {
	b.config.EncryptSelector = func([]SessionEncryption) SessionEncryption {
		return e
	}
	return b
}

func (b *ClientBuilder) ChannelBufferSize(bufferSize int) *ClientBuilder {
	b.config.ChannelBufferSize = bufferSize
	return b
}

func (b *ClientBuilder) Build() *Client {
	return NewClient(b.config, b.mux)
}
