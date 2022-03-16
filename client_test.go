package lime

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"log"
	"net"
	"testing"
	"time"
)

func TestClient_NewClient_Message(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := createLocalhostTCPAddress().(*net.TCPAddr)
	msgChan := make(chan *Message, 1)
	server := NewServerBuilder().
		ListenTCP(addr1, nil).
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
