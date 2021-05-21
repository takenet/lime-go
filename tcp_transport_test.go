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
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

func createListener(addr net.Addr, transportChan chan Transport, t *testing.T) *TCPTransportListener {
	listener := TCPTransportListener{}
	if err := listener.Open(context.Background(), addr); err != nil {
		t.Fatal(err)
		return nil
	}
	//listener.ReadTimeout = 5 * time.Second
	//listener.WriteTimeout = 5 * time.Second

	if transportChan != nil {
		go func() {
			for {
				t, err := listener.Accept()
				if err != nil {
					break
				}
				transportChan <- t
			}
		}()
	}

	return &listener
}

func createListenerTLS(addr net.Addr, transportChan chan Transport, t *testing.T) *TCPTransportListener {
	listener := createListener(addr, transportChan, t)
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}
	return listener
}

func createClientTransport(addr net.Addr, t *testing.T) *TCPTransport {
	client := TCPTransport{}
	if err := client.Open(context.Background(), addr); err != nil {
		t.Fatal(err)
		return nil
	}
	//client.WriteTimeout = 5 * time.Second
	//client.ReadTimeout = 5 * time.Second
	return &client
}

func createClientTransportTLS(addr net.Addr, t *testing.T) *TCPTransport {
	client := createClientTransport(addr, t)
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

func TestTCPTransport_Open_WhenListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createListener(addr, nil, t)
	defer listener.Close()

	client := TCPTransport{}

	// Act
	err := client.Open(context.Background(), addr)

	// Assert
	assert.Nil(t, err)
}

func TestTCPTransport_Open_WhenNotListening(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	client := TCPTransport{}

	// Act
	err := client.Open(context.Background(), addr)

	// Assert
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestTCPTransportListener_Close_WhenOpen(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createListener(addr, nil, t)
	defer listener.Close()
	client := createClientTransport(createTCPAddress(), t)

	// Act
	err := client.Close(context.TODO())

	// Assert
	assert.Nil(t, err)
}

func TestTCPTransportListener_Close_WhenNotOpen(t *testing.T) {
	// Arrange
	client := TCPTransport{}

	// Act
	err := client.Close(context.TODO())

	// Assert
	assert.NotNil(t, err)
	assert.Equal(t, "transport is not open", err.Error())
}

func TestTCPTransport_SetEncryption_None(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	listener := createListener(addr, nil, t)
	defer listener.Close()
	client := createClientTransport(createTCPAddress(), t)

	// Act
	err := client.SetEncryption(context.Background(), SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	addr := createTCPAddress()
	var transportChan = make(chan Transport, 1)
	listener := createListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTransportTLS(createTCPAddress(), t)
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
	listener := createListener(addr, nil, t)
	defer listener.Close()
	client := createClientTransport(createTCPAddress(), t)
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
	listener := createListener(addr, transportChan, t)
	defer listener.Close()
	client := createClientTransport(createTCPAddress(), t)
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
	listener := createListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTransportTLS(createTCPAddress(), t)
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
	listener := createListenerTLS(addr, transportChan, t)
	defer listener.Close()
	client := createClientTransportTLS(createTCPAddress(), t)
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
