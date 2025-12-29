package lime

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

const (
	wsURLFormat  = "ws://%s"
	wssURLFormat = "wss://%s"
)

func createWebsocketListener(ctx context.Context, t testing.TB, addr net.Addr, transportChan chan Transport) TransportListener {
	listener := NewWebsocketTransportListener(&WebsocketConfig{})
	if err := listener.Listen(ctx, addr); err != nil {
		t.Fatal(err)
		return nil
	}

	listenTransports(ctx, transportChan, listener)

	return listener
}

func listenTransports(ctx context.Context, transportChan chan Transport, listener TransportListener) {
	if transportChan != nil {
		go func() {
			for {
				t, err := listener.Accept(ctx)
				if err != nil {
					break
				}
				select {
				case <-ctx.Done():
					return
				case transportChan <- t:
				}
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

	listenTransports(ctx, transportChan, listener)

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

func createLocalhostWSAddr() net.Addr {
	return &net.TCPAddr{
		Port: 8080,
	}
}

func TestWebsocketTransportListenerAcceptWhenContextDeadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)

	// Act
	server, err := listener.Accept(ctx)

	// Assert
	assert.Nil(t, server)
	assert.Error(t, err)
	assert.Equal(t, "ws listener: context deadline exceeded", err.Error())
}

func TestWebsocketTransportListenerAcceptWhenClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
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

func TestWebsocketTransportDialWhenListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	url := fmt.Sprintf(wsURLFormat, addr)
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

func TestWebsocketTransportDialWhenNotListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	url := fmt.Sprintf(wsURLFormat, addr)

	// Act
	client, err := DialWebsocket(ctx, url, nil, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestWebsocketTransportDialAfterListenerClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	url := fmt.Sprintf(wsURLFormat, addr)
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

func TestWebsocketTransportCloseWhenOpen(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransportCloseWhenAlreadyClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
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

func TestWebsocketTransportSetEncryptionNone(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
	client := createClientWebsocketTransport(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionNone, client.Encryption())
}

func TestWebsocketTransportSetEncryptionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionTLS, client.Encryption())
}

func TestWebsocketTransportSendSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListener(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
	client := createClientWebsocketTransport(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransportSendSessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
	client := createClientWebsocketTransportTLS(ctx, t, url)
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestWebsocketTransportSendDeadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	listener := createWebsocketListenerTLS(ctx, t, addr, nil)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
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

func TestWebsocketTransportReceiveSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListener(ctx, t, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
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

func TestWebsocketTransportReceiveSessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListenerTLS(ctx, t, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
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

func TestWebsocketTransportReceiveDeadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	addr := createLocalhostWSAddr()
	var transportChan = make(chan Transport, 1)
	listener := createWebsocketListenerTLS(ctx, t, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
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

func BenchmarkWebsocketTransportSendMessage(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createLocalhostWSAddr()
	var transportChan = make(chan Transport)
	listener := createWebsocketListener(ctx, b, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf(wsURLFormat, addr)
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

func BenchmarkWebsocketTransportSendMessageTLS(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createLocalhostWSAddr()
	var transportChan = make(chan Transport)
	listener := createWebsocketListenerTLS(ctx, b, addr, transportChan)
	defer silentClose(listener)
	url := fmt.Sprintf(wssURLFormat, addr)
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
