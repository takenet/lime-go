package client

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/takenet/lime-go/internal/inprocess"
	serverpkg "github.com/takenet/lime-go/server"
	httptransport "github.com/takenet/lime-go/transport/http"
	"go.uber.org/goleak"
)

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
