package lime

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"io"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func createTCPListener(t testing.TB, addr net.Addr, transportChan chan Transport) TransportListener {
	listener := NewTCPTransportListener(nil)
	if err := listener.Listen(context.Background(), addr); err != nil {
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

	return listener
}

func createTCPListenerTLS(t testing.TB, addr net.Addr, transportChan chan Transport) TransportListener {
	config := &TCPConfig{TLSConfig: &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}}

	listener := NewTCPTransportListener(config)
	if err := listener.Listen(context.Background(), addr); err != nil {
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

	return listener
}

func createClientTCPTransport(t testing.TB, addr net.Addr) Transport {
	client, err := DialTcp(context.Background(), addr, nil)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createClientTCPTransportTLS(t testing.TB, addr net.Addr) Transport {
	client, err := DialTcp(context.Background(), addr, &TCPConfig{TLSConfig: &tls.Config{ServerName: "127.0.0.1", InsecureSkipVerify: true}})
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func receiveTransport(t testing.TB, transportChan chan Transport) Transport {
	select {
	case s := <-transportChan:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("Transport listener timeout")
	}
	//goland:noinspection GoUnreachableCode
	panic("something very wrong has occurred")
}

func createLocalhostTCPAddress() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
	}
}

func publicKey(p interface{}) interface{} {
	switch k := p.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func createCertificate(host string) (*tls.Certificate, error) {
	key, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 180),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(key), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, err
	}

	rawCertificate := [][]byte{cert.Raw}

	return &tls.Certificate{
		Certificate: rawCertificate,
		PrivateKey:  key,
		Leaf:        cert,
	}, nil
}

func doTLSHandshake(ctx context.Context, server Transport, client Transport) error {
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return server.SetEncryption(ctx, SessionEncryptionTLS)
	})

	if err := client.SetEncryption(ctx, SessionEncryptionTLS); err != nil {
		return err
	}

	return eg.Wait()
}

func TestTCPTransportListener_Accept_WhenContextDeadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// Act
	server, err := listener.Accept(ctx)

	// Assert
	assert.Nil(t, server)
	assert.Error(t, err)
	assert.Equal(t, "tcp listener: context deadline exceeded", err.Error())
}

func TestTCPTransportListener_Accept_WhenClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = listener.Close()
	}()
	server, err := listener.Accept(ctx)

	// Assert
	assert.Nil(t, server)
	assert.Error(t, err)
	assert.Equal(t, "tcp listener closed", err.Error())
}

func TestTCPTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, nil)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.Connected())
}

func TestTCPTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestTCPTransport_Dial_AfterListenerClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestTCPTransport_Close_WhenOpen(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientTCPTransport(t, createLocalhostTCPAddress())
	defer silentClose(client)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Close_WhenAlreadyClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientTCPTransport(t, createLocalhostTCPAddress())
	defer silentClose(client)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	err := client.Close()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestTCPTransport_Close_WhenNotOpen(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client := tcpTransport{}

	// Act
	err := client.Close()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestTCPTransport_SetEncryption_None(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientTCPTransport(t, createLocalhostTCPAddress())
	defer silentClose(client)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionNone, client.Encryption())
}

func TestTCPTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(t, createLocalhostTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionTLS, client.Encryption())
}

func TestTCPTransport_Send_Session(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientTCPTransport(t, createLocalhostTCPAddress())
	defer silentClose(client)
	s := createSession()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Send_SessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(t, createLocalhostTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Send_Deadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(t, createLocalhostTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	s := createSession()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestTCPTransport_Receive_Session(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListener(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransport(t, createLocalhostTCPAddress())
	defer silentClose(client)
	server := receiveTransport(t, transportChan)
	s := createSession()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
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

func TestTCPTransport_Receive_SessionTLS(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(t, createLocalhostTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}

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

func TestTCPTransport_Receive_Deadline(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(t, createLocalhostTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithDeadline(context.Background(), time.Now())
	defer cancel()

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.Nil(t, e)
	assert.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func BenchmarkTCPTransport_Send_Message(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListener(b, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransport(b, createLocalhostTCPAddress())
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

func BenchmarkTCPTransport_Send_MessageTLS(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addr := createLocalhostTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(b, addr, transportChan)
	defer silentClose(listener)
	client := createClientTCPTransportTLS(b, createLocalhostTCPAddress())
	server := receiveTransport(b, transportChan)
	if err := doTLSHandshake(ctx, server, client); err != nil {
		b.Fatal(err)
	}
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

func silentClose(c io.Closer) {
	_ = c.Close()
}
