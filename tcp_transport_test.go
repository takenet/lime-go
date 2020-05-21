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
	"encoding/pem"
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func openListener(addr net.Addr) (*TCPTransportListener, error) {
	listener := TCPTransportListener{}
	if err := listener.Open(context.Background(), addr); err != nil {
		return nil, err
	}
	return &listener, nil
}

func openClient(addr net.Addr) (*TCPTransport, error) {
	client := TCPTransport{}
	if err := client.Open(context.Background(), addr); err != nil {
		return nil, err
	}
	client.WriteTimeout = 5 * time.Second
	client.ReadTimeout = 5 * time.Second

	return &client, nil
}

func createAddress() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
		Zone: "",
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

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
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
	//if *isCA {
	//	template.IsCA = true
	//	template.KeyUsage |= x509.KeyUsageCertSign
	//}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(key), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, err
	}

	rawCertificate := [][]byte{cert.Raw}

	return &tls.Certificate{Certificate: rawCertificate}, nil

}

func TestTCPTransport_Open_WhenListening(t *testing.T) {
	// Arrange
	addr := createAddress()
	listener, err := openListener(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	client := TCPTransport{}

	// Act
	err = client.Open(context.Background(), addr)

	// Assert
	assert.Nil(t, err)
}

func TestTCPTransport_Open_WhenNotListening(t *testing.T) {
	// Arrange
	addr := createAddress()
	client := TCPTransport{}

	// Act
	err := client.Open(context.Background(), addr)

	// Assert
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestTCPTransportListener_Close_WhenOpen(t *testing.T) {
	// Arrange
	addr := createAddress()
	listener, err := openListener(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := openClient(createAddress())
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = client.Close()

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
	addr := createAddress()
	listener, err := openListener(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	client, err := openClient(createAddress())
	if err != nil {
		t.Fatal(err)
	}

	// Act
	err = client.SetEncryption(SessionEncryptionNone)

	// Assert
	assert.NoError(t, err)
}

func TestTCPTransport_SetEncryption_TLS(t *testing.T) {
	// Arrange
	addr := createAddress()
	listener, err := openListener(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	listener.TLSConfig = &tls.Config{

		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate("127.0.0.1")
		},
	}

	client, err := openClient(createAddress())
	if err != nil {
		t.Fatal(err)
	}
	client.TLSConfig = &tls.Config{ServerName: "127.0.0.1"}

	// Act
	err = client.SetEncryption(SessionEncryptionTLS)

	// Assert
	assert.NoError(t, err)
}
