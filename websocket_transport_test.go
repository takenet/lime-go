package lime

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func createWebsocketListener(ctx context.Context, t *testing.T, addr string, transportChan chan Transport) *WebsocketTransportListener {
	listener := WebsocketTransportListener{}
	if err := listener.Listen(ctx, addr); err != nil {
		t.Fatal(err)
		return nil
	}

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

	return &listener
}

func createWebsocketListenerTLS(ctx context.Context, t *testing.T, addr string, transportChan chan Transport) *WebsocketTransportListener {
	listener := createWebsocketListener(ctx, t, addr, transportChan)
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}
	return listener
}

func createClientWebsocketTransport(t *testing.T, urlStr string) Transport {
	client, err := DialWebsocket(context.Background(), urlStr, nil, nil)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createClientWebsocketTransportTLS(t *testing.T, addr string) Transport {
	client, err := DialWebsocket(context.Background(), addr, nil, &tls.Config{ServerName: "127.0.0.1", InsecureSkipVerify: true})
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func TestWebsocketTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := ":8080"
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
	addr := ":8080"
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
	addr := ":8080"
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
	addr := ":8080"
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(t, url)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Close_WhenAlreadyClosed(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	addr := ":8080"
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer listener.Close()
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(t, url)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	err := client.Close()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}
