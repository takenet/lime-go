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
	"golang.org/x/sync/errgroup"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func createTCPListener(t *testing.T, addr net.Addr, transportChan chan Transport) *TCPTransportListener {
	listener := TCPTransportListener{}
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

	return &listener
}

func createTCPListenerTLS(t *testing.T, addr net.Addr, transportChan chan Transport) *TCPTransportListener {
	listener := createTCPListener(t, addr, transportChan)
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}
	return listener
}

func createClientTCPTransport(t *testing.T, addr net.Addr) *TCPTransport {
	client, err := DialTcp(context.Background(), addr, &tls.Config{})
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func createClientTCPTransportTLS(t *testing.T, addr net.Addr) *TCPTransport {
	client := createClientTCPTransport(t, addr)
	client.TLSConfig = &tls.Config{ServerName: "127.0.0.1", InsecureSkipVerify: true}
	return client
}

func receiveTransport(t *testing.T, transportChan chan Transport) Transport {
	select {
	case s := <-transportChan:
		return s
	case <-time.After(5 * time.Second):
		t.Fatal("Transport listener timeout")
	}
	panic("something very wrong has occurred")
}

func createTCPAddress() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
	}
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
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

func TestTCPTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, &tls.Config{})

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.True(t, client.IsConnected())

}

func TestTCPTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, &tls.Config{})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestTCPTransport_Dial_AfterListenerClosed(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	client, err := DialTcp(ctx, addr, &tls.Config{})

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
	assert.Nil(t, client)
}

func TestTCPTransport_Close_WhenOpen(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer listener.Close()
	client := createClientTCPTransport(t, createTCPAddress())

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Close_WhenAlreadyClosed(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer listener.Close()
	client := createClientTCPTransport(t, createTCPAddress())
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
	client := TCPTransport{}

	// Act
	err := client.Close()

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestTCPTransport_SetEncryption_None(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer listener.Close()
	client := createClientTCPTransport(t, createTCPAddress())
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionNone, client.GetEncryption())
}

func TestTCPTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer listener.Close()
	client := createClientTCPTransportTLS(t, createTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := doTLSHandshake(ctx, server, client); err != nil {
		t.Fatal(err)
	}

	// Act
	err := client.SetEncryption(ctx, SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionEncryptionTLS, client.GetEncryption())
}

func TestTCPTransport_Send_Session(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(t, addr, nil)
	defer listener.Close()
	client := createClientTCPTransport(t, createTCPAddress())
	s := createSession()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	err := client.Send(ctx, s)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Receive_Session(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListener(t, addr, transportChan)
	defer listener.Close()
	client := createClientTCPTransport(t, createTCPAddress())
	server := receiveTransport(t, transportChan)
	s := createSession()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

func TestTCPTransport_Send_SessionTLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer listener.Close()
	client := createClientTCPTransportTLS(t, createTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

func TestTCPTransport_Receive_SessionTLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(t, addr, transportChan)
	defer listener.Close()
	client := createClientTCPTransportTLS(t, createTCPAddress())
	server := receiveTransport(t, transportChan)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
