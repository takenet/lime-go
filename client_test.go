package lime

import (
	"context"
	"errors"
	"log"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestClientNewClientMessage(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := createLocalhostTCPAddress().(*net.TCPAddr)
	msgChan := make(chan *Message, 1)
	server := NewServerBuilder().
		ListenTCP(addr1, nil).
		EnableGuestAuthentication().
		MessagesHandlerFunc(
			func(ctx context.Context, msg *Message, s Sender) error {
				msgChan <- msg
				return nil
			}).
		Build()
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Println(err)
		}
	}()
	config := NewClientConfig()
	config.EncryptSelector = NoneEncryptionSelector
	config.NewTransport = func(ctx context.Context) (Transport, error) {
		return DialTcp(ctx, addr1, nil)
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
	result := builder.Domain("example.com")

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, "example.com", builder.config.Node.Domain)
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
	result := builder.Instance("test-instance")

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, "test-instance", builder.config.Node.Instance)
}

func TestClientBuilderUseTCP(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}

	// Act
	result := builder.UseTCP(addr, nil)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.NewTransport)
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

func TestClientBuilderUseInProcess(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()
	addr := InProcessAddr("test-addr")

	// Act
	result := builder.UseInProcess(addr, 10)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.NewTransport)
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
	assert.NotEmpty(t, builder.mux.msgHandlers)
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
	assert.NotEmpty(t, builder.mux.msgHandlers)
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
	assert.NotEmpty(t, builder.mux.notHandlers)
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
	assert.NotEmpty(t, builder.mux.notHandlers)
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
	assert.NotEmpty(t, builder.mux.reqCmdHandlers)
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
	assert.NotEmpty(t, builder.mux.reqCmdHandlers)
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
	assert.NotEmpty(t, builder.mux.respCmdHandlers)
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
	assert.NotEmpty(t, builder.mux.respCmdHandlers)
}

func TestClientBuilderAutoReplyPings(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.AutoReplyPings()

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.reqCmdHandlers)
}

func TestClientBuilderBuild(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	builder := NewClientBuilder().
		Name("testuser").
		Domain("example.com").
		Instance("test-instance").
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
	assert.Equal(t, "example.com", client.config.Node.Domain)
	assert.Equal(t, "test-instance", client.config.Node.Instance)
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
	assert.NotEmpty(t, client.mux.msgHandlers)
	assert.NotEmpty(t, client.mux.notHandlers)
	assert.NotEmpty(t, client.mux.reqCmdHandlers)
}

func TestClientBuilderChaining(t *testing.T) {
	// Test that all builder methods support chaining
	defer goleak.VerifyNone(t)
	client := NewClientBuilder().
		Name("user").
		Domain("domain.com").
		Instance("inst1").
		GuestAuthentication().
		UseTCP(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080}, nil).
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

func TestClientEstablish(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addr := InProcessAddr("test-establish")
	server := NewServerBuilder().
		ListenInProcess(addr).
		EnableGuestAuthentication().
		Build()
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Println(err)
		}
	}()
	time.Sleep(100 * time.Millisecond) // Wait for server to start
	client := NewClientBuilder().
		Name("testclient").
		Domain("localhost").
		GuestAuthentication().
		UseInProcess(addr, 10).
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		Build()
	defer silentClose(client)

	// Act
	err := client.Establish(ctx)

	// Assert
	assert.NoError(t, err)
}

func TestClientSendNotification(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addr := InProcessAddr("test-notification")
	notChan := make(chan *Notification, 1)
	server := NewServerBuilder().
		ListenInProcess(addr).
		EnableGuestAuthentication().
		NotificationsHandlerFunc(
			func(ctx context.Context, not *Notification) error {
				notChan <- not
				return nil
			}).
		Build()
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Println(err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	client := NewClientBuilder().
		Name("testclient").
		Domain("localhost").
		GuestAuthentication().
		UseInProcess(addr, 10).
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		Build()
	defer silentClose(client)
	notification := &Notification{
		Envelope: Envelope{ID: "123"},
		Event:    NotificationEventReceived,
	}

	// Act
	err := client.SendNotification(ctx, notification)

	// Assert
	assert.NoError(t, err)
	select {
	case rcvNot := <-notChan:
		assert.Equal(t, notification.ID, rcvNot.ID)
		assert.Equal(t, notification.Event, rcvNot.Event)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for notification")
	}
}

func TestClientSendRequestCommand(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addr := InProcessAddr("test-send-command")
	cmdChan := make(chan *RequestCommand, 1)
	server := NewServerBuilder().
		ListenInProcess(addr).
		EnableGuestAuthentication().
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *RequestCommand, s Sender) error {
				cmdChan <- cmd
				return s.SendResponseCommand(ctx, cmd.SuccessResponse())
			}).
		Build()
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Println(err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	client := NewClientBuilder().
		Name("testclient").
		Domain("localhost").
		GuestAuthentication().
		UseInProcess(addr, 10).
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		Build()
	defer silentClose(client)
	uri, _ := ParseLimeURI("/test")
	command := &RequestCommand{
		Command: Command{
			Envelope: Envelope{ID: "cmd-123"},
			Method:   CommandMethodGet,
		},
		URI: uri,
	}

	// Act
	err := client.SendRequestCommand(ctx, command)

	// Assert
	assert.NoError(t, err)
	select {
	case rcvCmd := <-cmdChan:
		assert.Equal(t, command.ID, rcvCmd.ID)
		assert.Equal(t, command.Method, rcvCmd.Method)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for command")
	}
}

func TestClientProcessCommand(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	addr := InProcessAddr("test-process-command")
	server := NewServerBuilder().
		ListenInProcess(addr).
		EnableGuestAuthentication().
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *RequestCommand, s Sender) error {
				return s.SendResponseCommand(ctx, cmd.SuccessResponse())
			}).
		Build()
	defer silentClose(server)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, ErrServerClosed) {
			log.Println(err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
	client := NewClientBuilder().
		Name("testclient").
		Domain("localhost").
		GuestAuthentication().
		UseInProcess(addr, 10).
		Compression(SessionCompressionNone).
		Encryption(SessionEncryptionNone).
		Build()
	defer silentClose(client)
	uri, _ := ParseLimeURI("/ping")
	command := &RequestCommand{
		Command: Command{
			Envelope: Envelope{ID: "process-123"},
			Method:   CommandMethodGet,
		},
		URI: uri,
	}

	// Act
	response, err := client.ProcessCommand(ctx, command)

	// Assert
	if assert.NoError(t, err) {
		assert.NotNil(t, response)
		assert.Equal(t, CommandStatusSuccess, response.Status)
		assert.Equal(t, command.ID, response.ID)
	}
}
