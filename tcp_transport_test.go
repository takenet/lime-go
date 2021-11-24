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

func createTCPListener(addr net.Addr, transportChan chan Transport, t *testing.T) *TCPTransportListener {
	listener := TCPTransportListener{}
	if err := listener.Listen(context.Background(), addr); err != nil {
		t.Fatal(err)
		return nil
	}
	//listener.ReadTimeout = 5 * time.Second
	//listener.WriteTimeout = 5 * time.Second

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

func createTCPListenerTLS(addr net.Addr, transportChan chan Transport, t *testing.T) *TCPTransportListener {
	listener := createTCPListener(addr, transportChan, t)
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}
	return listener
}

func createClientTCPTransport(addr net.Addr, t *testing.T) *TCPTransport {

	client, err := DialTcp(context.Background(), addr, &tls.Config{})

	if err != nil {
		t.Fatal(err)
		return nil
	}
	//client.WriteTimeout = 5 * time.Second
	//client.ReadTimeout = 5 * time.Second
	return client
}

func createClientTCPTransportTLS(addr net.Addr, t *testing.T) *TCPTransport {
	client := createClientTCPTransport(addr, t)
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
	panic("Something very wrong has occurred")
}

func createTCPAddress() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
		Zone: "",
	}
}

func createMessage() *Message {
	m := Message{}
	m.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = "limeprotocol.org"
	m.To.Instance = "default"
	var d PlainDocument = "Hello world"
	m.SetContent(&d)
	return &m
}

func createSession() *Session {
	s := Session{}
	s.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	s.From = Node{}
	s.From.Name = "postmaster"
	s.From.Domain = "limeprotocol.org"
	s.From.Instance = "#server1"
	s.To = Node{}
	s.To.Name = "golang"
	s.To.Domain = "limeprotocol.org"
	s.To.Instance = "default"
	s.State = SessionStateEstablished
	return &s
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

func TestTCPTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(addr, nil, t)
	defer listener.Close()

	// Act
	_, err := DialTcp(context.Background(), addr, &tls.Config{})

	// Assert
	assert.Nil(t, err)
}

func TestTCPTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()

	// Act
	_, err := DialTcp(context.Background(), addr, &tls.Config{})

	// Assert
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestTCPTransportListener_Close_WhenOpen(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(addr, nil, t)
	defer listener.Close()
	client := createClientTCPTransport(createTCPAddress(), t)

	// Act
	err := client.Close()

	// Assert
	assert.Nil(t, err)
}

func TestTCPTransportListener_Close_WhenNotOpen(t *testing.T) {
	// Arrange
	client := TCPTransport{}

	// Act
	err := client.Close()

	// Assert
	assert.NotNil(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestTCPTransport_SetEncryption_None(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(addr, nil, t)
	defer listener.Close()
	client := createClientTCPTransport(createTCPAddress(), t)

	// Act
	err := client.SetEncryption(context.Background(), SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTCPTransportTLS(createTCPAddress(), t)
	server := receiveTransport(t, transportChan)
	go func() {
		if err := server.SetEncryption(context.Background(), SessionEncryptionTLS); err != nil {
			t.Fatal(err)
		}
	}()

	// Act
	err := client.SetEncryption(context.Background(), SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Send_Session(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createTCPListener(addr, nil, t)
	defer listener.Close()
	client := createClientTCPTransport(createTCPAddress(), t)
	s := createSession()

	// Act
	err := client.Send(context.Background(), s)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Receive_Session(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListener(addr, transportChan, t)
	defer listener.Close()
	client := createClientTCPTransport(createTCPAddress(), t)
	server := receiveTransport(t, transportChan)
	s := createSession()
	if err := client.Send(context.Background(), s); err != nil {
		t.Fatal(err)
	}

	// Act
	e, err := server.Receive(context.Background())

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
	listener := createTCPListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTCPTransportTLS(createTCPAddress(), t)
	server := receiveTransport(t, transportChan)
	go func() {
		if err := server.SetEncryption(context.Background(), SessionEncryptionTLS); err != nil {
			t.Fatal(err)
		}
	}()
	if err := client.SetEncryption(context.Background(), SessionEncryptionTLS); err != nil {
		t.Fatal(err)
	}
	s := createSession()

	// Act
	err := client.Send(context.Background(), s)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_Receive_SessionTLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createTCPListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTCPTransportTLS(createTCPAddress(), t)
	server := receiveTransport(t, transportChan)

	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return server.SetEncryption(ctx, SessionEncryptionTLS)
	})

	if err := client.SetEncryption(ctx, SessionEncryptionTLS); err != nil {
		t.Fatal(err)
	}

	if err := eg.Wait(); err != nil {
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
