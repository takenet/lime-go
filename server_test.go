package lime

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"net"
	"testing"
	"time"
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

func TestServer_ListenAndServe_WithMultipleListeners(t *testing.T) {
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

func TestServer_ListenAndServe_EstablishSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	config := NewServerConfig()
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
	client, err := DialInProcess(addr1, 1)
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

func TestServer_ListenAndServe_ReceiveMessage(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	config := NewServerConfig()
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

func TestServerBuilder_Build(t *testing.T) {
	// Arrange
	//builder := NewServerBuilder().

}
