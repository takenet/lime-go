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
	assert.Equal(t, "testuser", builder.config.Node.Identity.Name)
}

func TestClientBuilderDomain(t *testing.T) {
	// Arrange
	builder := NewClientBuilder()

	// Act
	result := builder.Domain("example.com")

	// Assert
	assert.Equal(t, builder, result)
	assert.Equal(t, "example.com", builder.config.Node.Identity.Domain)
}

func TestNewClientConfig(t *testing.T) {
	// Act
	config := NewClientConfig()

	// Assert
	assert.NotEmpty(t, config.Node.Identity.Name)
	assert.NotNil(t, config.Authenticator)
	assert.NotZero(t, config.ChannelBufferSize)
}
