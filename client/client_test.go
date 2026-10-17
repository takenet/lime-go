package client

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	channelpkg "github.com/takenet/lime-go/channel"
	"github.com/takenet/lime-go/internal/inprocess"
	"github.com/takenet/lime-go/protocol"
	serverpkg "github.com/takenet/lime-go/server"
	httptransport "github.com/takenet/lime-go/transport/http"
	"go.uber.org/goleak"
)

type lifecycleTransport struct {
	Transport
	sendErr    error
	closeErr   error
	closeCount atomic.Int32
	closed     atomic.Bool
	receive    func(context.Context) (protocol.EnvelopeValue, error)
	send       func(context.Context, protocol.EnvelopeValue) error
	close      func()
}

func (transport *lifecycleTransport) Connected() bool { return !transport.closed.Load() }

func (transport *lifecycleTransport) Send(ctx context.Context, envelope protocol.EnvelopeValue) error {
	if transport.send != nil {
		return transport.send(ctx, envelope)
	}
	return transport.sendErr
}

func (transport *lifecycleTransport) Receive(ctx context.Context) (protocol.EnvelopeValue, error) {
	return transport.receive(ctx)
}

func (transport *lifecycleTransport) Close() error {
	transport.closeCount.Add(1)
	if transport.close != nil {
		transport.close()
	}
	transport.closed.Store(true)
	return transport.closeErr
}

func TestClientBuildChannel_EstablishmentFailure_ClosesTransport(t *testing.T) {
	establishErr := errors.New("establishment failed")
	for _, closeErr := range []error{nil, errors.New("close failed")} {
		t.Run(fmt.Sprint(closeErr), func(t *testing.T) {
			transport := &lifecycleTransport{sendErr: establishErr, closeErr: closeErr}
			config := NewClientConfig()
			config.NewTransport = func(context.Context) (Transport, error) { return transport, nil }
			client := &Client{config: config}

			channel, err := client.buildChannel(context.Background())

			require.ErrorIs(t, err, establishErr)
			assert.Nil(t, channel)
			assert.EqualValues(t, 1, transport.closeCount.Load())
		})
	}
}

func TestClientBuildChannel_UnsuccessfulSession_ClosesTransport(t *testing.T) {
	receiveErr := errors.New("receive failed")
	tests := []struct {
		name    string
		receive func(context.Context) (protocol.EnvelopeValue, error)
		wantErr error
	}{
		{
			name: "receive failure",
			receive: func(context.Context) (protocol.EnvelopeValue, error) {
				return nil, receiveErr
			},
			wantErr: receiveErr,
		},
		{
			name: "cancellation",
			receive: func(ctx context.Context) (protocol.EnvelopeValue, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			wantErr: context.Canceled,
		},
		{
			name: "non-established session",
			receive: func(context.Context) (protocol.EnvelopeValue, error) {
				return &protocol.Session{State: protocol.SessionStateNew}, nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			transport := &lifecycleTransport{receive: test.receive}
			if test.wantErr == context.Canceled {
				transport.send = func(context.Context, protocol.EnvelopeValue) error {
					cancel()
					return nil
				}
			}
			config := NewClientConfig()
			config.NewTransport = func(context.Context) (Transport, error) { return transport, nil }
			client := &Client{config: config}

			channel, err := client.buildChannel(ctx)

			require.Error(t, err)
			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
			} else {
				assert.Contains(t, err.Error(), "channel state is new")
			}
			assert.Nil(t, channel)
			assert.EqualValues(t, 1, transport.closeCount.Load())
		})
	}
}

func TestClientLocalNode(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	config := NewClientConfig()
	config.Node = Node{Identity: Identity{Name: "requested", Domain: "localhost"}, Instance: "requested"}
	acceptedNodes := []Node{
		{Identity: Identity{Name: "accepted", Domain: "server.example"}, Instance: "first"},
		{Identity: Identity{Name: "replacement", Domain: "server.example"}, Instance: "second"},
	}
	var transport *lifecycleTransport
	buildCount := 0
	config.NewTransport = func(context.Context) (Transport, error) {
		sessions := make(chan protocol.EnvelopeValue, 1)
		sessions <- &protocol.Session{
			Envelope: protocol.Envelope{To: acceptedNodes[buildCount]},
			State:    SessionStateEstablished,
		}
		buildCount++
		transport = &lifecycleTransport{
			receive: func(ctx context.Context) (protocol.EnvelopeValue, error) {
				select {
				case session := <-sessions:
					return session, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
			send: func(_ context.Context, envelope protocol.EnvelopeValue) error {
				if session := envelope.(*protocol.Session); session.State == protocol.SessionStateFinishing {
					sessions <- &protocol.Session{State: protocol.SessionStateFinished}
				}
				return nil
			},
		}
		return transport, nil
	}
	client := &Client{config: config, lock: make(chan struct{}, 1)}
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	node, ok := client.LocalNode()
	assert.False(t, ok)
	assert.Equal(t, Node{}, node)
	client.channel = channelpkg.NewClientChannel(&lifecycleTransport{}, 1)
	node, ok = client.LocalNode()
	assert.False(t, ok)
	assert.Equal(t, Node{}, node)

	require.NoError(t, client.Establish(ctx))
	node, ok = client.LocalNode()
	assert.True(t, ok)
	assert.Equal(t, acceptedNodes[0], node)
	assert.NotEqual(t, config.Node, node)

	stopReads := make(chan struct{})
	readsStarted := make(chan struct{})
	readsDone := make(chan struct{})
	go func() {
		defer close(readsDone)
		close(readsStarted)
		for {
			select {
			case <-stopReads:
				return
			default:
				node, ok := client.LocalNode()
				if ok {
					assert.Contains(t, acceptedNodes, node)
				} else {
					assert.Equal(t, Node{}, node)
				}
			}
		}
	}()
	t.Cleanup(func() {
		close(stopReads)
		<-readsDone
	})
	<-readsStarted

	require.NoError(t, transport.Close())
	node, ok = client.LocalNode()
	assert.False(t, ok)
	assert.Equal(t, Node{}, node)
	require.NoError(t, client.Establish(ctx))
	node, ok = client.LocalNode()
	assert.True(t, ok)
	assert.Equal(t, acceptedNodes[1], node)
	require.NoError(t, client.Close())
	node, ok = client.LocalNode()
	assert.False(t, ok)
	assert.Equal(t, Node{}, node)
}

func TestClientClose_BlockingTransport_DetachesWithoutHoldingMutex(t *testing.T) {
	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseClose) })
	closeErr := errors.New("close failed")
	transport := &lifecycleTransport{closeErr: closeErr, close: func() {
		close(closeStarted)
		<-releaseClose
	}}
	client := &Client{channel: channelpkg.NewClientChannel(transport, 1), lock: make(chan struct{}, 1)}
	closeResult := make(chan error, 1)
	go func() { closeResult <- client.Close() }()
	t.Cleanup(release)

	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("transport close did not start")
	}
	require.True(t, client.mu.TryRLock(), "Close must not hold mu during transport shutdown")
	assert.Nil(t, client.channel)
	client.mu.RUnlock()
	node, ok := client.LocalNode()
	assert.False(t, ok)
	assert.Equal(t, Node{}, node)
	assert.Len(t, client.lock, 1, "Close must retain the lifecycle lock")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	channel, err := client.getOrBuildChannel(ctx)
	assert.Nil(t, channel)
	assert.ErrorIs(t, err, context.Canceled)

	release()
	require.ErrorIs(t, <-closeResult, closeErr)
	require.NoError(t, client.Close())
	assert.EqualValues(t, 1, transport.closeCount.Load())
}

func TestClientClose_ConcurrentBuild_ClosesPublishedChannel(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	buildStarted := make(chan struct{})
	releaseBuild := make(chan struct{})
	sessions := make(chan protocol.EnvelopeValue, 1)
	sessions <- &protocol.Session{State: SessionStateEstablished}
	transport := &lifecycleTransport{}
	transport.receive = func(ctx context.Context) (protocol.EnvelopeValue, error) {
		select {
		case session := <-sessions:
			return session, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	transport.send = func(_ context.Context, envelope protocol.EnvelopeValue) error {
		if session := envelope.(*protocol.Session); session.State == protocol.SessionStateFinishing {
			sessions <- &protocol.Session{State: protocol.SessionStateFinished}
		}
		return nil
	}
	config := NewClientConfig()
	config.NewTransport = func(ctx context.Context) (Transport, error) {
		close(buildStarted)
		select {
		case <-releaseBuild:
			return transport, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	client := &Client{config: config, lock: make(chan struct{}, 1)}
	stopReads := make(chan struct{})
	readsDone := make(chan struct{})
	go func() {
		defer close(readsDone)
		for {
			select {
			case <-stopReads:
				return
			default:
				_ = client.establishedChannel()
				_, _ = client.LocalNode()
			}
		}
	}()
	t.Cleanup(func() {
		close(stopReads)
		<-readsDone
	})
	buildResult := make(chan error, 1)
	go func() { buildResult <- client.Establish(ctx) }()
	select {
	case <-buildStarted:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	closeResult := make(chan error, 1)
	go func() { closeResult <- client.Close() }()
	select {
	case err := <-closeResult:
		t.Fatalf("Close returned before the build completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(releaseBuild)
	require.NoError(t, <-buildResult)
	require.NoError(t, <-closeResult)
	assert.Nil(t, client.establishedChannel())
	assert.EqualValues(t, 1, transport.closeCount.Load())
}

func TestClientNewClientMessage(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("client-new-client-message")
	msgChan := make(chan *Message, 1)
	serverConfig := serverpkg.NewServerConfig()
	serverConfig.SchemeOpts = []serverpkg.AuthenticationScheme{serverpkg.AuthenticationSchemeGuest}
	serverMux := &serverpkg.EnvelopeMux{}
	serverMux.MessageHandlerFunc(func(*serverpkg.Message) bool { return true }, func(ctx context.Context, msg *serverpkg.Message, sender serverpkg.Sender) error {
		msgChan <- msg
		return nil
	})
	server := serverpkg.NewServer(
		serverConfig,
		serverMux,
		serverpkg.NewBoundListener(inprocess.NewInProcessTransportListener(addr1), addr1),
	)
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, serverpkg.ErrServerClosed) {
			log.Println(err)
		}
	}()
	config := NewClientConfig()
	config.EncryptSelector = NoneEncryptionSelector
	config.NewTransport = func(context.Context) (Transport, error) {
		return inprocess.DialInProcess(addr1, 1)
	}
	mux := &EnvelopeMux{}
	client := NewClient(config, mux)
	msg := createMessage()

	// Act
	err := client.SendMessage(ctx, msg)

	// Assert
	assert.NoError(t, err)
	rcvMsg := <-msgChan
	assert.Equal(t, msg, rcvMsg)
	err = client.Close()
	assert.NoError(t, err)
}

func TestNewClientBuilder(t *testing.T) {
	// Act
	builder := NewClientBuilder()

	// Assert
	assert.NotNil(t, builder)
}

func TestClientBuilderName(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Name("testuser")

	// Assert
	assert.Equal(t, builder, result) // should return self for chaining
	assert.Equal(t, "testuser", builder.config.Node.Name)
}

func TestClientBuilderDomain(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Domain(testExampleDomain)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, testExampleDomain, builder.config.Node.Domain)
}

func TestNewClientConfig(t *testing.T) {
	// Act
	config := NewClientConfig()

	// Assert
	assert.NotEmpty(t, config.Node.Name)
	assert.NotNil(t, config.Authenticator)
	assert.NotZero(t, config.ChannelBufferSize)
}

func TestClientBuilderInstance(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Instance(testClientInstance)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, testClientInstance, builder.config.Node.Instance)
}

func TestClientBuilderUseWebsocket(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.UseWebsocket("ws://localhost:8080", nil, nil)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.NewTransport)
}

func TestClientBuilderUseHTTPTransport(t *testing.T) {
	builder := NewClientBuilder().
		Name("iris-client").
		Domain(testExampleDomain).
		UseHTTPTransport(httptransport.Config{BaseURL: "https://iris.example.com"})

	assert.NotNil(t, builder.config.NewTransport)
	authentication := builder.config.Authenticator(
		[]AuthenticationScheme{AuthenticationSchemeTransport},
		nil,
	)
	assert.IsType(t, &TransportAuthentication{}, authentication)

	configuredTransport, err := builder.config.NewTransport(context.Background())
	if assert.NoError(t, err) {
		assert.True(t, configuredTransport.Connected())
		_ = configuredTransport.Close()
	}
}

func TestClientBuilderGuestAuthentication(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.GuestAuthentication()

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Authenticator)

	// Test authenticator returns GuestAuthentication
	auth := builder.config.Authenticator([]AuthenticationScheme{AuthenticationSchemeGuest}, nil)
	_, ok := auth.(*GuestAuthentication)
	assert.True(t, ok)
}

func TestClientBuilderTransportAuthentication(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.TransportAuthentication()

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Authenticator)

	// Test authenticator returns TransportAuthentication
	auth := builder.config.Authenticator([]AuthenticationScheme{AuthenticationSchemeTransport}, nil)
	_, ok := auth.(*TransportAuthentication)
	assert.True(t, ok)
}

func TestClientBuilderPlainAuthentication(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.PlainAuthentication("mypassword")

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Authenticator)

	// Test authenticator returns PlainAuthentication with password
	auth := builder.config.Authenticator([]AuthenticationScheme{AuthenticationSchemePlain}, nil)
	plainAuth, ok := auth.(*PlainAuthentication)
	assert.True(t, ok)
	assert.NotEmpty(t, plainAuth.Password)
}

func TestClientBuilderKeyAuthentication(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.KeyAuthentication("mykey")

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Authenticator)

	// Test authenticator returns KeyAuthentication with key
	auth := builder.config.Authenticator([]AuthenticationScheme{AuthenticationSchemeKey}, nil)
	keyAuth, ok := auth.(*KeyAuthentication)
	assert.True(t, ok)
	assert.NotEmpty(t, keyAuth.Key)
}

func TestClientBuilderExternalAuthentication(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.ExternalAuthentication("token123", "issuer-domain")

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Authenticator)

	// Test authenticator returns ExternalAuthentication
	auth := builder.config.Authenticator([]AuthenticationScheme{AuthenticationSchemeExternal}, nil)
	extAuth, ok := auth.(*ExternalAuthentication)
	assert.True(t, ok)
	assert.Equal(t, "token123", extAuth.Token)
	assert.Equal(t, "issuer-domain", extAuth.Issuer)
}

func TestClientBuilderCompression(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Compression(SessionCompressionGzip)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.CompSelector)

	// Test compression selector returns the configured compression
	comp := builder.config.CompSelector([]SessionCompression{SessionCompressionNone, SessionCompressionGzip})
	assert.Equal(t, SessionCompressionGzip, comp)
}

func TestClientBuilderEncryption(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Encryption(SessionEncryptionTLS)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.EncryptSelector)

	// Test encryption selector returns the configured encryption
	enc := builder.config.EncryptSelector([]SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS})
	assert.Equal(t, SessionEncryptionTLS, enc)
}

func TestClientBuilderChannelBufferSize(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.ChannelBufferSize(256)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, 256, builder.config.ChannelBufferSize)
}

func TestClientBuilderMessageHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	predicate := func(msg *Message) bool { return msg.Content != nil }
	handler := func(ctx context.Context, msg *Message, s Sender) error {
		return nil
	}

	// Act
	result := builder.MessageHandlerFunc(predicate, handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().Messages)
}

func TestClientBuilderMessagesHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	handler := func(ctx context.Context, msg *Message, s Sender) error {
		return nil
	}

	// Act
	result := builder.MessagesHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().Messages)
}

func TestClientBuilderNotificationHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	predicate := func(not *Notification) bool { return not.Event == NotificationEventReceived }
	handler := NotificationHandlerFunc(func(ctx context.Context, not *Notification) error {
		return nil
	})

	// Act
	result := builder.NotificationHandlerFunc(predicate, handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().Notifications)
}

func TestClientBuilderNotificationsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	handler := NotificationHandlerFunc(func(ctx context.Context, not *Notification) error {
		return nil
	})

	// Act
	result := builder.NotificationsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().Notifications)
}

func TestClientBuilderRequestCommandHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	predicate := func(cmd *RequestCommand) bool { return cmd.Method == CommandMethodGet }
	handler := func(ctx context.Context, cmd *RequestCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.RequestCommandHandlerFunc(predicate, handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().RequestCommands)
}

func TestClientBuilderRequestCommandsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	handler := func(ctx context.Context, cmd *RequestCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.RequestCommandsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().RequestCommands)
}

func TestClientBuilderResponseCommandHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	predicate := func(cmd *ResponseCommand) bool { return cmd.Status == CommandStatusSuccess }
	handler := func(ctx context.Context, cmd *ResponseCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.ResponseCommandHandlerFunc(predicate, handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().ResponseCommands)
}

func TestClientBuilderResponseCommandsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	handler := func(ctx context.Context, cmd *ResponseCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.ResponseCommandsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().ResponseCommands)
}

func TestClientBuilderAutoReplyPings(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.AutoReplyPings()

	// Assert
	assert.Equal(t, builder, result)
	assert.NotZero(t, builder.mux.HandlerCounts().RequestCommands)
}

func TestClientBuilderBuild(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	builder := NewClientBuilder().
		Name("testuser").
		Domain(testExampleDomain).
		Instance(testClientInstance).
		GuestAuthentication().
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		ChannelBufferSize(100)

	// Act
	client := builder.Build()
	defer func() { _ = client.Close() }()

	// Assert
	assert.NotNil(t, client)
	assert.Equal(t, "testuser", client.config.Node.Name)
	assert.Equal(t, testExampleDomain, client.config.Node.Domain)
	assert.Equal(t, testClientInstance, client.config.Node.Instance)
	assert.Equal(t, 100, client.config.ChannelBufferSize)
}

func TestClientBuilderBuildWithHandlers(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	builder := NewClientBuilder().
		MessagesHandlerFunc(func(ctx context.Context, msg *Message, s Sender) error {
			return nil
		}).
		NotificationsHandlerFunc(NotificationHandlerFunc(func(ctx context.Context, not *Notification) error {
			return nil
		})).
		RequestCommandsHandlerFunc(func(ctx context.Context, cmd *RequestCommand, s Sender) error {
			return nil
		})

	// Act
	client := builder.Build()
	defer func() { _ = client.Close() }()

	// Assert
	assert.NotNil(t, client)
	assert.NotZero(t, client.mux.HandlerCounts().Messages)
	assert.NotZero(t, client.mux.HandlerCounts().Notifications)
	assert.NotZero(t, client.mux.HandlerCounts().RequestCommands)
}

func TestClientBuilderChaining(t *testing.T) {
	// Test that all builder methods support chaining
	defer goleak.VerifyNone(t)
	client := NewClientBuilder().
		Name("user").
		Domain("domain.com").
		Instance("inst1").
		GuestAuthentication().
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		ChannelBufferSize(50).
		AutoReplyPings().
		Build()
	defer func() { _ = client.Close() }()

	assert.NotNil(t, client)
	assert.Equal(t, "user", client.config.Node.Name)
	assert.Equal(t, "domain.com", client.config.Node.Domain)
	assert.Equal(t, "inst1", client.config.Node.Instance)
	assert.Equal(t, 50, client.config.ChannelBufferSize)
}
