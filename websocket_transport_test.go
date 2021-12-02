package lime

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
	"time"
)

func createWebsocketListener(ctx context.Context, t *testing.T, addr net.Addr, transportChan chan Transport) *WebsocketTransportListener {
	listener := &WebsocketTransportListener{}
	if err := listener.Listen(ctx, addr); err != nil {
		t.Fatal(err)
		return nil
	}

	listenTransports(transportChan, listener)

	return listener
}

func listenTransports(transportChan chan Transport, listener TransportListener) {
	if transportChan != nil {
		go func() {
			for {
				t, err := listener.Accept(context.Background())
				if err != nil {
					break
				}
				transportChan <- t
			}
		}()
	}
}

func createWebsocketListenerTLS(ctx context.Context, t *testing.T, addr net.Addr, transportChan chan Transport) *WebsocketTransportListener {
	listener := &WebsocketTransportListener{}
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}
	if err := listener.Listen(ctx, addr); err != nil {
		t.Fatal(err)
		return nil
	}

	listenTransports(transportChan, listener)

	return listener
}

func createClientWebsocketTransport(ctx context.Context, t *testing.T, urlStr string) Transport {
	client, err := DialWebsocket(ctx, urlStr, nil, nil)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createClientWebsocketTransportTLS(ctx context.Context, t *testing.T, addr string) Transport {
	client, err := DialWebsocket(ctx, addr, nil, &tls.Config{ServerName: "127.0.0.1", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createWSAddr() net.Addr {
	return &net.TCPAddr{
		Port: 8080,
	}
}

func TestWebsocketTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	url := fmt.Sprintf("ws://%s", addr)
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()

	// Act
	client, err := DialWebsocket(ctx, url, nil, nil)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.IsConnected())
}

func TestWebsocketTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	url := fmt.Sprintf("ws://%s", addr)

	// Act
	client, err := DialWebsocket(ctx, url, nil, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestWebsocketTransport_Dial_AfterListenerClosed(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	url := fmt.Sprintf("ws://%s", addr)
	listener := createWebsocketListener(ctx, t, addr, nil)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	client, err := DialWebsocket(ctx, url, nil, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestWebsocketTransport_Close_WhenOpen(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Close_WhenAlreadyClosed(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	err := client.Close()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestWebsocketTransport_SetEncryption_None(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionNone, client.GetEncryption())
}

func TestWebsocketTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionTLS, client.GetEncryption())
}

func TestWebsocketTransport_Send_Session(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Receive_Session(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListener(ctx, t, addr, transportChan)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)
	server := receiveTransport(t, transportChan)
	s := createSession()
	if err := client.Send(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.NoError(t, err)
	received, ok := e.(*Session)
	assert.True(t, ok)
	assert.Equal(t, s, received)
}

func TestWebsocketTransport_Send_SessionTLS(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Receive_SessionTLS(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListenerTLS(ctx, t, addr, transportChan)
	defer listener.Close()
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	server := receiveTransport(t, transportChan)
	s := createSession()
	if err := client.Send(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.NoError(t, err)
	received, ok := e.(*Session)
	assert.True(t, ok)
	assert.Equal(t, s, received)
}
