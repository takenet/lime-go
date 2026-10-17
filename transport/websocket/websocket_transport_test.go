package websocket

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/protocol"
	"go.uber.org/goleak"
)

const (
	wsURLFormat    = "ws://%s"
	wssURLFormat   = "wss://%s"
	testCommandID  = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	testDomain     = "limeprotocol.org"
	testHelloWorld = "Hello world"
)

func createSession() *protocol.Session {
	return &protocol.Session{State: protocol.SessionStateNew}
}

type observedWebsocketConn struct {
	net.Conn
	observeRead  atomic.Bool
	blockWrite   atomic.Bool
	readStarted  chan struct{}
	writeStarted chan struct{}
	closed       chan struct{}
	readOnce     sync.Once
	writeOnce    sync.Once
	closeOnce    sync.Once
}

func (conn *observedWebsocketConn) Read(buffer []byte) (int, error) {
	if conn.observeRead.Load() {
		conn.readOnce.Do(func() { close(conn.readStarted) })
	}
	return conn.Conn.Read(buffer)
}

func (conn *observedWebsocketConn) Write(buffer []byte) (int, error) {
	if conn.blockWrite.Load() {
		conn.writeOnce.Do(func() { close(conn.writeStarted) })
		<-conn.closed
		return 0, net.ErrClosed
	}
	return conn.Conn.Write(buffer)
}

func (conn *observedWebsocketConn) Close() error {
	conn.closeOnce.Do(func() { close(conn.closed) })
	return conn.Conn.Close()
}

func createObservedWebsocketPair(t *testing.T) (*websocketTransport, *websocketTransport, *observedWebsocketConn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		accepted <- conn
	}))
	t.Cleanup(server.Close)
	var observed *observedWebsocketConn
	dialer := websocket.Dialer{NetDialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		observed = &observedWebsocketConn{
			Conn: conn, readStarted: make(chan struct{}), writeStarted: make(chan struct{}), closed: make(chan struct{}),
		}
		return observed, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, _, err := dialer.DialContext(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	client := &websocketTransport{conn: conn}
	t.Cleanup(func() { silentClose(client) })
	select {
	case peerConn := <-accepted:
		peer := &websocketTransport{conn: peerConn}
		t.Cleanup(func() { silentClose(peer) })
		return client, peer, observed
	case <-ctx.Done():
		t.Fatal("websocket upgrade timeout")
		return nil, nil, nil
	}
}

func waitWebsocketSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatal("websocket operation did not start")
	}
}

func waitWebsocketResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("websocket operation did not finish")
		return nil
	}
}

func TestWebsocketTransportCanceledIOClosesConnection(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	for _, read := range []bool{true, false} {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("read=%t/deadline=%t", read, deadline), func(t *testing.T) {
				client, peer, observed := createObservedWebsocketPair(t)
				observed.observeRead.Store(true)
				observed.blockWrite.Store(true)
				ctx, cancel := context.WithCancel(context.Background())
				wantErr := context.Canceled
				if deadline {
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					wantErr = context.DeadlineExceeded
				}
				defer cancel()
				result := make(chan error, 1)
				go func() {
					if read {
						_, err := client.Receive(ctx)
						result <- err
					} else {
						result <- client.Send(ctx, createSession())
					}
				}()
				if read {
					waitWebsocketSignal(t, observed.readStarted)
				} else {
					waitWebsocketSignal(t, observed.writeStarted)
				}
				if !deadline {
					cancel()
				}
				require.ErrorIs(t, waitWebsocketResult(t, result), wantErr)
				assert.False(t, client.Connected())
				assert.Error(t, client.Send(context.Background(), createSession()))
				_, err := client.Receive(context.Background())
				assert.Error(t, err)
				peerCtx, peerCancel := context.WithTimeout(context.Background(), time.Second)
				defer peerCancel()
				_, err = peer.Receive(peerCtx)
				assert.Error(t, err)
				assert.False(t, peer.Connected())
			})
		}
	}
}

func TestWebsocketTransportQueuedCancellationPreservesConnection(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	for _, read := range []bool{true, false} {
		t.Run(fmt.Sprintf("read=%t", read), func(t *testing.T) {
			client, peer, _ := createObservedWebsocketPair(t)
			require.NoError(t, client.acquire(context.Background(), read))
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			result := make(chan error, 1)
			go func() {
				if read {
					_, err := client.Receive(ctx)
					result <- err
				} else {
					result <- client.Send(ctx, createSession())
				}
			}()
			require.ErrorIs(t, waitWebsocketResult(t, result), context.DeadlineExceeded)
			assert.True(t, client.Connected())
			if read {
				<-client.readGate
			} else {
				<-client.writeGate
			}
			require.NoError(t, peer.Send(context.Background(), createSession()))
			_, err := client.Receive(context.Background())
			require.NoError(t, err)
			require.NoError(t, client.Send(context.Background(), createSession()))
			_, err = peer.Receive(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestWebsocketTransportIOFailureDisconnects(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	for _, read := range []bool{true, false} {
		t.Run(fmt.Sprintf("read=%t", read), func(t *testing.T) {
			client, _, observed := createObservedWebsocketPair(t)
			require.NoError(t, observed.Close())
			if read {
				_, err := client.Receive(context.Background())
				require.ErrorIs(t, err, net.ErrClosed)
			} else {
				require.ErrorIs(t, client.Send(context.Background(), createSession()), net.ErrClosed)
			}
			assert.False(t, client.Connected())
		})
	}
}

func TestWebsocketTransportPreCanceledIOPreservesConnection(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	client, peer, _ := createObservedWebsocketPair(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, client.Send(ctx, createSession()), context.Canceled)
	_, err := client.Receive(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, client.Connected())
	require.NoError(t, peer.Send(context.Background(), createSession()))
	_, err = client.Receive(context.Background())
	require.NoError(t, err)
	require.NoError(t, client.Send(context.Background(), createSession()))
	_, err = peer.Receive(context.Background())
	require.NoError(t, err)
}

func TestWebsocketTransportConcurrentIO(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	client, peer, _ := createObservedWebsocketPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	const count = 20
	result := make(chan error, count*4)
	for range count {
		for _, endpoint := range []*websocketTransport{client, peer} {
			go func() { result <- endpoint.Send(ctx, createSession()) }()
			go func() {
				envelope, err := endpoint.Receive(ctx)
				if err == nil && envelope == nil {
					err = fmt.Errorf("received nil envelope")
				}
				result <- err
			}()
		}
	}
	for range count * 4 {
		require.NoError(t, waitWebsocketResult(t, result))
	}
	assert.True(t, client.Connected())
	assert.True(t, peer.Connected())
}

func TestWebsocketTransportConcurrentCloseUnblocksIO(t *testing.T) {
	t.Cleanup(func() { goleak.VerifyNone(t) })
	client, _, observed := createObservedWebsocketPair(t)
	observed.observeRead.Store(true)
	observed.blockWrite.Store(true)
	result := make(chan error, 4)
	for range 2 {
		go func() {
			_, err := client.Receive(context.Background())
			result <- err
		}()
		go func() { result <- client.Send(context.Background(), createSession()) }()
	}
	waitWebsocketSignal(t, observed.readStarted)
	waitWebsocketSignal(t, observed.writeStarted)
	var closers sync.WaitGroup
	for range 20 {
		closers.Go(func() {
			_ = client.Connected()
			_ = client.LocalAddr()
			_ = client.RemoteAddr()
			_ = client.Close()
		})
	}
	closers.Wait()
	for range 4 {
		require.Error(t, waitWebsocketResult(t, result))
	}
	assert.False(t, client.Connected())
}

func createMessage() *protocol.Message {
	message := &protocol.Message{Envelope: protocol.Envelope{ID: testCommandID}}
	message.SetTo(protocol.Node{
		Identity: protocol.Identity{Name: "golang", Domain: testDomain},
		Instance: "default",
	})
	content := protocol.TextDocument(testHelloWorld)
	message.SetContent(&content)
	return message
}

func silentClose(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}

func receiveTransport(t testing.TB, transports <-chan Transport) Transport {
	select {
	case received := <-transports:
		return received
	case <-time.After(5 * time.Second):
		t.Fatal("transport listener timeout")
		return nil
	}
}

func createWebsocketTestCertificate(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"LIME test"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, hostname := range strings.Split(host, ",") {
		if ip := net.ParseIP(hostname); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, hostname)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &tls.Certificate{
		Certificate: [][]byte{certificate.Raw},
		PrivateKey:  key,
		Leaf:        certificate,
	}, nil
}

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
			return createWebsocketTestCertificate("127.0.0.1")
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr := listener.Addr()
	_ = listener.Close()
	return addr
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
	received, ok := e.(*protocol.Session)
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
	received, ok := e.(*protocol.Session)
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
	messages := make([]*protocol.Message, b.N)
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
	messages := make([]*protocol.Message, b.N)
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
