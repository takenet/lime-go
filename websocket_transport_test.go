package lime

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"net"
	"testing"
	"time"
)

func createWebsocketListener(ctx context.Context, t testing.TB, addr net.Addr, transportChan chan Transport) TransportListener {
	listener := NewWebsocketTransportListener(&WebsocketConfig{})
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

func createWebsocketListenerTLS(ctx context.Context, t testing.TB, addr net.Addr, transportChan chan Transport) TransportListener {
	listener := NewWebsocketTransportListener(&WebsocketConfig{TLSConfig: &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}})
	if err := listener.Listen(ctx, addr); err != nil {
		t.Fatal(err)
		return nil
	}

	listenTransports(transportChan, listener)

	return listener
}

func createClientWebsocketTransport(ctx context.Context, t testing.TB, urlStr string) Transport {
	client, err := DialWebsocket(ctx, urlStr, nil, nil)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createClientWebsocketTransportTLS(ctx context.Context, t testing.TB, addr string) Transport {
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

func TestWebsocketTransportListener_Accept_WhenContextDeadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)

	// Act
	server, err := listener.Accept(ctx)

	// Assert
	assert.Nil(t, server)
	assert.Error(t, err)
	assert.Equal(t, "ws listener: context deadline exceeded", err.Error())
}

func TestWebsocketTransportListener_Accept_WhenClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)

	// Act
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = listener.Close()
	}()
	server, err := listener.Accept(ctx)

	// Assert
	assert.Nil(t, server)
	assert.Error(t, err)
	assert.Equal(t, "ws listener closed", err.Error())
}

func TestWebsocketTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	url := fmt.Sprintf("ws://%s", addr)
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)

	// Act
	client, err := DialWebsocket(ctx, url, nil, nil)
	defer silentClose(client)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.Connected())
}

func TestWebsocketTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
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
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
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
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Close_WhenAlreadyClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
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
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionNone, client.Encryption())
}

func TestWebsocketTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionTLS, client.Encryption())
}

func TestWebsocketTransport_Send_Session(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Send_SessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransport_Send_Deadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	s := createSession()
	ctx, cancel = context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestWebsocketTransport_Receive_Session(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListener(ctx, t, addr, transportChan)
	defer silentClose(listener)
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

func TestWebsocketTransport_Receive_SessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListenerTLS(ctx, t, addr, transportChan)
	defer silentClose(listener)
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

func TestWebsocketTransport_Receive_Deadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListenerTLS(ctx, t, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	defer silentClose(client)
	server := receiveTransport(t, transportChan)
	ctx, cancel = context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.Nil(t, e)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func BenchmarkWebsocketTransport_Send_Message(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport)
	listener := createWebsocketListener(ctx, b, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf("ws://%s", addr)
	client := createClientWebsocketTransport(ctx, b, url)
	server := receiveTransport(b, transportChan)
	messages := make([]*Message, b.N)
	for i := 0; i < len(messages); i++ {
		messages[i] = createMessage()
	}
	errChan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errChan <- err
				return
			}
		}
		done <- true
	}()
	for _, m := range messages {
		_ = client.Send(ctx, m)
	}
	select {
	case <-ctx.Done():
		b.Fatal(ctx.Err())
	case err := <-errChan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func BenchmarkWebsocketTransport_Send_MessageTLS(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createWSAddr()
	var transportChan = make(chan Transport)
	listener := createWebsocketListenerTLS(ctx, b, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf("wss://%s", addr)
	client := createClientWebsocketTransportTLS(ctx, b, url)
	server := receiveTransport(b, transportChan)
	messages := make([]*Message, b.N)
	for i := 0; i < len(messages); i++ {
		messages[i] = createMessage()
	}
	errChan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errChan <- err
				return
			}
		}
		done <- true
	}()
	for _, m := range messages {
		_ = client.Send(ctx, m)
	}
	select {
	case <-ctx.Done():
		b.Fatal(ctx.Err())
	case err := <-errChan:
		b.Fatal(err)
	case <-done:
		break
	}
}
