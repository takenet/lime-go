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
	"os"
	"os/signal"
	"strings"
	"time"
)

func main() {
	addr := net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55321,
		Zone: "",
	}

	l := createTCPListenerTLS(&addr)
	c := make(chan lime.Transport, 1)
	go func(l lime.TransportListener, c chan<- lime.Transport) {
		for {
			t, err := l.Accept(context.Background())
			if err != nil {
				log.Printf("transport accept failed: %v\n", err)
				break
			}
			c <- t
		}
	}(l, c)

	ctx, cancel := context.WithCancel(context.Background())
	go listen(ctx, c)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	fmt.Printf("Listening at %v. Press Ctrl+C to stop.\n", addr)
	<-sig

	cancel()
	if err := l.Close(); err != nil {
		log.Printf("listener stop failed: %v\n", err)
	}
}

var serverNode = lime.Node{Identity: lime.Identity{Name: "postmaster", Domain: "localhost"}, Instance: "server1"}

func listen(ctx context.Context, c <-chan lime.Transport) {
	for {
		select {
		case <-ctx.Done():
			log.Println("Listener stopped")
			return
		case t := <-c:
			if err := acceptTransport(ctx, t); err != nil {
				_ = t.Close()
				log.Printf("Accepting transport failed: %v\n", err)
			}
		}
	}
}

func acceptTransport(ctx context.Context, t lime.Transport) error {
	c := lime.NewServerChannel(t, 1, serverNode, lime.NewEnvelopeId())

	if err := c.EstablishSession(
		ctx,
		[]lime.SessionCompression{lime.SessionCompressionNone},
		[]lime.SessionEncryption{lime.SessionEncryptionNone},
		[]lime.AuthenticationScheme{lime.AuthenticationSchemeGuest},
		authenticate,
		register,
	); err != nil {
		log.Printf("Establish session failed: %v\n", err)
		return err
	}

	go listenChannel(ctx, c)
	return nil
}

func listenChannel(ctx context.Context, c *lime.ServerChannel) {
	for c.Established() {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.MsgChan():
			if msg == nil {
				return
			}
			fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
		case not := <-c.NotChan():
			if not == nil {
				return
			}
			fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
		case cmd := <-c.CmdChan():
			if cmd == nil {
				return
			}
			fmt.Printf("Command received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
		case ses := <-c.SesChan():
			if ses == nil {
				return
			}
			var err error
			if ses.State == lime.SessionStateFinishing {
				err = c.FinishSession(context.Background())
			} else {
				err = c.FailSession(context.Background(), &lime.Reason{
					Code:        1,
					Description: "Invalid session state",
				})
			}
			if err != nil {
				log.Printf("Error closing the session: %v\n", err)
			}
			return
		}
	}
}

func authenticate(lime.Identity, lime.Authentication) (lime.AuthenticationResult, error) {
	return lime.AuthenticationResult{Role: lime.DomainRoleMember}, nil
}

func register(lime.Node, *lime.ServerChannel) (lime.Node, error) {
	return lime.Node{Identity: lime.Identity{
		Name:   lime.NewEnvelopeId(),
		Domain: "localhost",
	}, Instance: lime.NewEnvelopeId()}, nil
}

func createTCPListenerTLS(addr net.Addr) lime.TransportListener {
	config := &lime.TCPConfig{TLSConfig: &tls.Config{
		GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return createCertificate(addr.String())
		},
	}}

	listener := lime.NewTCPTransportListener(config)
	if err := listener.Listen(context.Background(), addr); err != nil {
		log.Fatal(err)
		return nil
	}

	return listener
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
