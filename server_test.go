package lime

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
)

func createBoundInProcTransportListener(addr InProcessAddr) BoundListener {
	return BoundListener{
		Listener: NewInProcessTransportListener(addr),
		Addr:     addr,
	}
}

func createBoundTCPTransportListener(addr net.Addr) BoundListener {
	return BoundListener{
		Listener: NewTCPTransportListener(nil),
		Addr:     addr,
	}
}

func createBoundWSTransportListener(addr net.Addr) BoundListener {
	return BoundListener{
		Listener: NewWebsocketTransportListener(nil),
		Addr:     addr,
	}
}

func TestServerListenAndServeWithMultipleListeners(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	addr2 := createLocalhostTCPAddress()
	listener2 := createBoundTCPTransportListener(addr2)
	addr3 := createLocalhostWSAddr()
	listener3 := createBoundWSTransportListener(addr3)
	config := NewServerConfig()
	mux := &EnvelopeMux{}
	srv := NewServer(config, mux, listener1, listener2, listener3)
	done := make(chan bool)
	eg, _ := errgroup.WithContext(context.Background())

	// Act
	eg.Go(func() error {
		close(done)
		return srv.ListenAndServe()
	})
	<-done
	time.Sleep(16 * time.Millisecond)

	// Assert
	client1, err := DialInProcess(addr1, 1)
	assert.NoError(t, err)
	defer silentClose(client1)
	ses := &Session{State: SessionStateNew}
	err = client1.Send(ctx, ses)
	assert.NoError(t, err)
	client2, err := DialTcp(ctx, addr2, nil)
	assert.NoError(t, err)
	defer silentClose(client2)
	err = client2.Send(ctx, ses)
	assert.NoError(t, err)
	client3, err := DialWebsocket(ctx, fmt.Sprintf("ws://%s", addr3), nil, nil)
	assert.NoError(t, err)
	err = client3.Send(ctx, ses)
	defer silentClose(client3)
	assert.NoError(t, err)
	err = srv.Close()
	assert.NoError(t, err)
	assert.Error(t, eg.Wait(), ErrServerClosed)
}

func TestServerListenAndServeEstablishSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	config := NewServerConfig()
	config.SchemeOpts = []AuthenticationScheme{AuthenticationSchemeGuest}
	mux := &EnvelopeMux{}
	srv := NewServer(config, mux, listener1)
	defer silentClose(srv)
	done := make(chan bool)
	eg, _ := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		close(done)
		return srv.ListenAndServe()
	})

	// Act
	<-done
	time.Sleep(16 * time.Millisecond)
	client, _ := DialInProcess(addr1, 1)
	defer silentClose(client)
	channel := NewClientChannel(client, 1)
	defer silentClose(channel)
	ses, err := channel.EstablishSession(
		ctx,
		func([]SessionCompression) SessionCompression {
			return SessionCompressionNone
		},
		func([]SessionEncryption) SessionEncryption {
			return SessionEncryptionNone
		},
		Identity{
			Name:   "client1",
			Domain: "localhost",
		},
		func([]AuthenticationScheme, Authentication) Authentication {
			return &GuestAuthentication{}
		},
		"default")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionStateEstablished, ses.State)
}

func TestServerListenAndServeReceiveMessage(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	config := NewServerConfig()
	config.SchemeOpts = []AuthenticationScheme{AuthenticationSchemeGuest}
	msgChan := make(chan *Message)
	mux := &EnvelopeMux{}
	mux.MessageHandlerFunc(
		func(*Message) bool {
			return true
		},
		func(ctx context.Context, msg *Message, s Sender) error {
			msgChan <- msg
			return nil
		})

	srv := NewServer(config, mux, listener1)
	defer silentClose(srv)
	done := make(chan bool)
	eg, _ := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		close(done)
		return srv.ListenAndServe()
	})
	<-done
	time.Sleep(16 * time.Millisecond)
	client, _ := DialInProcess(addr1, 1)
	defer silentClose(client)
	channel := NewClientChannel(client, 1)
	defer silentClose(channel)
	_, _ = channel.EstablishSession(
		ctx,
		func([]SessionCompression) SessionCompression {
			return SessionCompressionNone
		},
		func([]SessionEncryption) SessionEncryption {
			return SessionEncryptionNone
		},
		Identity{
			Name:   "client1",
			Domain: "localhost",
		},
		func([]AuthenticationScheme, Authentication) Authentication {
			return &GuestAuthentication{}
		},
		"default")
	msg := createMessage()

	// Act
	err := channel.SendMessage(ctx, msg)

	// Assert
	assert.NoError(t, err)
	select {
	case <-ctx.Done():
		assert.FailNow(t, "receive message timeout")
	case receivedMsg := <-msgChan:
		assert.Equal(t, msg, receivedMsg)
	}
}

func TestServerBuilderBuild(t *testing.T) {
	// Arrange
	addr := InProcessAddr("test-server-basic")
	builder := NewServerBuilder().ListenInProcess(addr)

	// Act
	result := builder.Build()
	defer silentClose(result)

	// Assert
	assert.NotNil(t, result)
}

func TestServerBuilderName(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.Name("myserver")

	// Assert
	assert.Equal(t, builder, result) // should return self for chaining
	assert.Equal(t, "myserver", builder.config.Node.Name)
}

func TestServerBuilderDomain(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.Domain("example.com")

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, "example.com", builder.config.Node.Domain)
}

func TestServerBuilderInstance(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.Instance("server-instance-1")

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, "server-instance-1", builder.config.Node.Instance)
}

func TestServerBuilderListenTCP(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9090}

	// Act
	result := builder.ListenTCP(addr, nil)

	// Assert
	assert.Equal(t, builder, result)
	assert.Len(t, builder.listeners, 1)
	assert.Equal(t, addr, builder.listeners[0].Addr)
}

func TestServerBuilderListenWebsocket(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	addr := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 9091}

	// Act
	result := builder.ListenWebsocket(addr, nil)

	// Assert
	assert.Equal(t, builder, result)
	assert.Len(t, builder.listeners, 1)
	assert.Equal(t, addr, builder.listeners[0].Addr)
}

func TestServerBuilderListenInProcess(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	addr := InProcessAddr("test-server")

	// Act
	result := builder.ListenInProcess(addr)

	// Assert
	assert.Equal(t, builder, result)
	assert.Len(t, builder.listeners, 1)
	assert.Equal(t, addr, builder.listeners[0].Addr)
}

func TestServerBuilderCompressionOptions(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.CompressionOptions(SessionCompressionNone, SessionCompressionGzip)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, []SessionCompression{SessionCompressionNone, SessionCompressionGzip}, builder.config.CompOpts)
}

func TestServerBuilderEncryptionOptions(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.EncryptionOptions(SessionEncryptionNone, SessionEncryptionTLS)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}, builder.config.EncryptOpts)
}

func TestServerBuilderEnableGuestAuthentication(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	builder.config.SchemeOpts = []AuthenticationScheme{} // Reset to empty

	// Act
	result := builder.EnableGuestAuthentication()

	// Assert
	assert.Equal(t, builder, result)
	assert.Contains(t, builder.config.SchemeOpts, AuthenticationSchemeGuest)
}

func TestServerBuilderEnableTransportAuthentication(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	builder.config.SchemeOpts = []AuthenticationScheme{} // Reset to empty

	// Act
	result := builder.EnableTransportAuthentication()

	// Assert
	assert.Equal(t, builder, result)
	assert.Contains(t, builder.config.SchemeOpts, AuthenticationSchemeTransport)
}

func TestServerBuilderEnablePlainAuthentication(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	builder.config.SchemeOpts = []AuthenticationScheme{} // Reset to empty
	authenticator := func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}

	// Act
	result := builder.EnablePlainAuthentication(authenticator)

	// Assert
	assert.Equal(t, builder, result)
	assert.Contains(t, builder.config.SchemeOpts, AuthenticationSchemePlain)
	assert.NotNil(t, builder.plainAuth)
}

func TestServerBuilderEnableKeyAuthentication(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	builder.config.SchemeOpts = []AuthenticationScheme{} // Reset to empty
	authenticator := func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}

	// Act
	result := builder.EnableKeyAuthentication(authenticator)

	// Assert
	assert.Equal(t, builder, result)
	assert.Contains(t, builder.config.SchemeOpts, AuthenticationSchemeKey)
	assert.NotNil(t, builder.keyAuth)
}

func TestServerBuilderEnableExternalAuthentication(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	builder.config.SchemeOpts = []AuthenticationScheme{} // Reset to empty
	authenticator := func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}

	// Act
	result := builder.EnableExternalAuthentication(authenticator)

	// Assert
	assert.Equal(t, builder, result)
	assert.Contains(t, builder.config.SchemeOpts, AuthenticationSchemeExternal)
	assert.NotNil(t, builder.externalAuth)
}

func TestServerBuilderChannelBufferSize(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.ChannelBufferSize(256)

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, 256, builder.config.ChannelBufferSize)
}

func TestServerBuilderMessageHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
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

func TestServerBuilderMessagesHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	handler := func(ctx context.Context, msg *Message, s Sender) error {
		return nil
	}

	// Act
	result := builder.MessagesHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.msgHandlers)
}

func TestServerBuilderNotificationHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
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

func TestServerBuilderNotificationsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	handler := NotificationHandlerFunc(func(ctx context.Context, not *Notification) error {
		return nil
	})

	// Act
	result := builder.NotificationsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.notHandlers)
}

func TestServerBuilderRequestCommandHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
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

func TestServerBuilderRequestCommandsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	handler := func(ctx context.Context, cmd *RequestCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.RequestCommandsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.reqCmdHandlers)
}

func TestServerBuilderResponseCommandHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
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

func TestServerBuilderResponseCommandsHandlerFunc(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	handler := func(ctx context.Context, cmd *ResponseCommand, s Sender) error {
		return nil
	}

	// Act
	result := builder.ResponseCommandsHandlerFunc(handler)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.respCmdHandlers)
}

func TestServerBuilderAutoReplyPings(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()

	// Act
	result := builder.AutoReplyPings()

	// Assert
	assert.Equal(t, builder, result)
	assert.NotEmpty(t, builder.mux.reqCmdHandlers)
}

func TestServerBuilderRegister(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	registerFunc := func(ctx context.Context, candidate Node, c *ServerChannel) (Node, error) {
		return candidate, nil
	}

	// Act
	result := builder.Register(registerFunc)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Register)
}

func TestServerBuilderEstablished(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	establishedFunc := func(sessionID string, c *ServerChannel) {}

	// Act
	result := builder.Established(establishedFunc)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Established)
}

func TestServerBuilderFinished(t *testing.T) {
	// Arrange
	builder := NewServerBuilder()
	finishedFunc := func(sessionID string) {}

	// Act
	result := builder.Finished(finishedFunc)

	// Assert
	assert.Equal(t, builder, result)
	assert.NotNil(t, builder.config.Finished)
}

func TestServerBuilderBuildWithAllOptions(t *testing.T) {
	// Arrange
	addr := InProcessAddr("test-server-full")
	builder := NewServerBuilder().
		Name("myserver").
		Domain("example.com").
		Instance("inst-1").
		ListenInProcess(addr).
		EnableGuestAuthentication().
		EnableTransportAuthentication().
		CompressionOptions(SessionCompressionNone).
		EncryptionOptions(SessionEncryptionNone).
		ChannelBufferSize(128)

	// Act
	server := builder.Build()
	defer silentClose(server)

	// Assert
	assert.NotNil(t, server)
	assert.Equal(t, "myserver", server.config.Node.Name)
	assert.Equal(t, "example.com", server.config.Node.Domain)
	assert.Equal(t, "inst-1", server.config.Node.Instance)
	assert.Equal(t, 128, server.config.ChannelBufferSize)
	assert.Len(t, server.listeners, 1)
	assert.Contains(t, server.config.SchemeOpts, AuthenticationSchemeGuest)
	assert.Contains(t, server.config.SchemeOpts, AuthenticationSchemeTransport)
}

func TestServerBuilderBuildWithHandlers(t *testing.T) {
	// Arrange
	addr := InProcessAddr("test-server-handlers")
	builder := NewServerBuilder().
		ListenInProcess(addr).
		EnableGuestAuthentication().
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
	server := builder.Build()
	defer silentClose(server)

	// Assert
	assert.NotNil(t, server)
	assert.NotEmpty(t, server.mux.msgHandlers)
	assert.NotEmpty(t, server.mux.notHandlers)
	assert.NotEmpty(t, server.mux.reqCmdHandlers)
}

func TestServerBuilderChaining(t *testing.T) {
	// Test that all builder methods support chaining
	addr := InProcessAddr("test-server-chain")
	server := NewServerBuilder().
		Name("server").
		Domain("domain.com").
		Instance("inst2").
		ListenInProcess(addr).
		EnableGuestAuthentication().
		CompressionOptions(SessionCompressionNone).
		EncryptionOptions(SessionEncryptionNone).
		ChannelBufferSize(64).
		AutoReplyPings().
		Build()
	defer silentClose(server)

	assert.NotNil(t, server)
	assert.Equal(t, "server", server.config.Node.Name)
	assert.Equal(t, "domain.com", server.config.Node.Domain)
	assert.Equal(t, "inst2", server.config.Node.Instance)
	assert.Equal(t, 64, server.config.ChannelBufferSize)
}

func TestAuthenticatePlainWithPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	plainAuth := func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error) {
		if password == "correct-password" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	plainAuthentication := &PlainAuthentication{}
	plainAuthentication.SetPasswordAsBase64("correct-password")

	// Act
	result, err := authenticatePlain(ctx, identity, plainAuthentication, plainAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleMember, result.Role)
}

func TestAuthenticatePlainWithWrongPassword(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	plainAuth := func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error) {
		if password == "correct-password" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	plainAuthentication := &PlainAuthentication{}
	plainAuthentication.SetPasswordAsBase64("wrong-password")

	// Act
	result, err := authenticatePlain(ctx, identity, plainAuthentication, plainAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleUnknown, result.Role)
}

func TestAuthenticatePlainWithNilAuthenticator(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	plainAuthentication := &PlainAuthentication{}
	plainAuthentication.SetPasswordAsBase64("password")

	// Act
	result, err := authenticatePlain(ctx, identity, plainAuthentication, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "plain authenticator is nil")
}

func TestAuthenticateKeyWithValidKey(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	keyAuth := func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error) {
		if key == "valid-key-123" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	keyAuthentication := &KeyAuthentication{}
	keyAuthentication.SetKeyAsBase64("valid-key-123")

	// Act
	result, err := authenticateKey(ctx, identity, keyAuthentication, keyAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleMember, result.Role)
}

func TestAuthenticateKeyWithInvalidKey(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	keyAuth := func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error) {
		if key == "valid-key-123" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	keyAuthentication := &KeyAuthentication{}
	keyAuthentication.SetKeyAsBase64("invalid-key")

	// Act
	result, err := authenticateKey(ctx, identity, keyAuthentication, keyAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleUnknown, result.Role)
}

func TestAuthenticateKeyWithNilAuthenticator(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	keyAuthentication := &KeyAuthentication{}
	keyAuthentication.SetKeyAsBase64("key")

	// Act
	result, err := authenticateKey(ctx, identity, keyAuthentication, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "key authenticator is nil")
}

func TestAuthenticateExternalWithValidToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	externalAuth := func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error) {
		if token == "valid-token" && issuer == "trusted-issuer" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	externalAuthentication := &ExternalAuthentication{
		Token:  "valid-token",
		Issuer: "trusted-issuer",
	}

	// Act
	result, err := authenticateExternal(ctx, identity, externalAuthentication, externalAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleMember, result.Role)
}

func TestAuthenticateExternalWithInvalidToken(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	externalAuth := func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error) {
		if token == "valid-token" && issuer == "trusted-issuer" {
			return MemberAuthenticationResult(), nil
		}
		return UnknownAuthenticationResult(), nil
	}
	externalAuthentication := &ExternalAuthentication{
		Token:  "invalid-token",
		Issuer: "trusted-issuer",
	}

	// Act
	result, err := authenticateExternal(ctx, identity, externalAuthentication, externalAuth)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, DomainRoleUnknown, result.Role)
}

func TestAuthenticateExternalWithNilAuthenticator(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	externalAuthentication := &ExternalAuthentication{
		Token:  "token",
		Issuer: "issuer",
	}

	// Act
	result, err := authenticateExternal(ctx, identity, externalAuthentication, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "external authenticator is nil")
}

func TestBuildAuthenticateWithAllSchemes(t *testing.T) {
	// Arrange
	ctx := context.Background()
	identity := Identity{Name: "user", Domain: "example.com"}
	plainAuth := func(ctx context.Context, identity Identity, password string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}
	keyAuth := func(ctx context.Context, identity Identity, key string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}
	externalAuth := func(ctx context.Context, identity Identity, token string, issuer string) (*AuthenticationResult, error) {
		return MemberAuthenticationResult(), nil
	}
	authenticate := buildAuthenticate(plainAuth, keyAuth, externalAuth)

	// Test Guest authentication - requires UUID format identity
	guestIdentity := Identity{Name: "550e8400-e29b-41d4-a716-446655440000", Domain: "example.com"}
	guestAuth := &GuestAuthentication{}
	result, err := authenticate(ctx, guestIdentity, guestAuth)
	assert.NoError(t, err)
	assert.Equal(t, DomainRoleMember, result.Role)

	// Test Plain authentication
	plainAuthentication := &PlainAuthentication{}
	plainAuthentication.SetPasswordAsBase64("password")
	result, err = authenticate(ctx, identity, plainAuthentication)
	assert.NoError(t, err)
	assert.Equal(t, DomainRoleMember, result.Role)

	// Test Key authentication
	keyAuthentication := &KeyAuthentication{}
	keyAuthentication.SetKeyAsBase64("key")
	result, err = authenticate(ctx, identity, keyAuthentication)
	assert.NoError(t, err)
	assert.Equal(t, DomainRoleMember, result.Role)

	// Test External authentication
	externalAuthentication := &ExternalAuthentication{Token: "token", Issuer: "issuer"}
	result, err = authenticate(ctx, identity, externalAuthentication)
	assert.NoError(t, err)
	assert.Equal(t, DomainRoleMember, result.Role)
}
