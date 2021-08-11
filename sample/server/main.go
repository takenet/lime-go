package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/takenet/lime-go"
	"log"
	"math/big"
	"net"
	"strings"
	"time"
)

func main() {
	addr := net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
		Zone: "",
	}

	l := createListenerTLS(&addr)
	tchan := make(chan lime.Transport, 1)

	go func(l lime.TransportListener, c chan<- lime.Transport) {
		for {
			t, err := l.Accept()
			if err != nil {
				log.Printf("transport accept failed: %v\n", err)
				break
			}
			c <- t
		}
	}(l, tchan)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Println("Listener stopped")
				return
			case t := <-tchan:
				if err := acceptTransport(t); err != nil {
					break
				}
			}
		}
	}()

	fmt.Printf("Listening at %v. Press ENTER to stop.\n", addr)
	_, _ = fmt.Scanln()
	cancel()
	if err := l.Close(); err != nil {
		log.Printf("listener stop failed: %v\n", err)
	}

}

var serverNode lime.Node = lime.Node{Identity: lime.Identity{
	Name:   "postmaster",
	Domain: "localhost",
},
	Instance: "server1"}

func acceptTransport(t lime.Transport) error {

	c, err := lime.NewServerChannel(t, 1, serverNode, lime.NewEnvelopeId())
	if err != nil {
		log.Printf("create channel failed: %v\n", err)
		return err
	}

	if err = c.EstablishSession(
		context.Background(),
		[]lime.SessionCompression{lime.SessionCompressionNone},
		[]lime.SessionEncryption{lime.SessionEncryptionNone},
		[]lime.AuthenticationScheme{lime.AuthenticationSchemeGuest},
		authenticate,
		register,
	); err != nil {
		log.Printf("establish session failed: %v\n", err)
	}

	return err
}

func authenticate(lime.Identity, lime.Authentication) (lime.AuthenticationResult, error) {
	return lime.AuthenticationResult{Role: lime.DomainRoleMember}, nil
}

func register(n lime.Node, c *lime.ServerChannel) (lime.Node, error) {
	return lime.Node{Identity: lime.Identity{
		Name:   lime.NewEnvelopeId(),
		Domain: "localhost",
	}, Instance: lime.NewEnvelopeId()}, nil
}

func createListenerTLS(addr net.Addr) *lime.TCPTransportListener {
	listener := createListener(addr)
	listener.TLSConfig = &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate(addr.String())
		},
	}
	return listener
}

func createListener(addr net.Addr) *lime.TCPTransportListener {
	listener := lime.TCPTransportListener{}
	if err := listener.Listen(context.Background(), addr); err != nil {
		log.Fatal(err)
		return nil
	}

	return &listener
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
