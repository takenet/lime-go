package lime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type TCPTransport struct {
	ConnTransport
	// The configuration for TLS session encryption
	TLSConfig  *tls.Config
	encryption SessionEncryption
	server     bool
}

func (t *TCPTransport) Open(ctx context.Context, addr net.Addr) error {
	if t.conn != nil {
		return errors.New("transport already open")
	}

	if addr.Network() != "tcp" {
		return errors.New("address network should be tcp")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, addr.Network(), addr.String())
	if err != nil {
		return err
	}

	t.setConn(conn)
	t.encryption = SessionEncryptionNone
	return nil
}

func (t *TCPTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *TCPTransport) GetCompression() SessionCompression {
	return SessionCompressionNone
}

func (t *TCPTransport) SetCompression(c SessionCompression) error {
	return fmt.Errorf("compression '%v' is not supported", c)
}

func (t *TCPTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}
}

func (t *TCPTransport) GetEncryption() SessionEncryption {
	return t.encryption
}

func (t *TCPTransport) SetEncryption(e SessionEncryption) error {
	if e == t.encryption {
		return nil
	}

	if e == SessionEncryptionNone {
		return errors.New("cannot downgrade from tls to none encryption")
	}

	if e == SessionEncryptionTLS && t.TLSConfig == nil {
		return errors.New("tls config must be defined")
	}

	var tlsConn *tls.Conn

	// https://github.com/FluuxIO/go-xmpp/blob/master/xmpp_transport.go#L80
	if t.server {
		tlsConn = tls.Server(t.conn, t.TLSConfig)
	} else {
		tlsConn = tls.Client(t.conn, t.TLSConfig)
	}

	if err := tlsConn.SetWriteDeadline(time.Now().Add(t.WriteTimeout)); err != nil {
		return err
	}

	if err := tlsConn.SetReadDeadline(time.Now().Add(t.ReadTimeout)); err != nil {
		return err
	}

	// We convert existing connection to TLS
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	t.setConn(tlsConn)
	t.encryption = SessionEncryptionTLS
	return nil
}

type TCPTransportListener struct {
	ConnTransportConfig
	TLSConfig *tls.Config
	listener  net.Listener
	mux       sync.Mutex
}

func (t *TCPTransportListener) Open(ctx context.Context, addr net.Addr) error {
	if addr.Network() != "tcp" {
		return errors.New("address network should be tcp")
	}

	t.mux.Lock()
	defer t.mux.Unlock()

	if t.listener != nil {
		return errors.New("listener is already started")
	}

	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", addr.String())
	if err != nil {
		return err
	}

	t.listener = l
	return nil
}

func (t *TCPTransportListener) Close() error {
	t.mux.Lock()
	defer t.mux.Unlock()

	if t.listener == nil {
		return errors.New("listener is not started")
	}

	err := t.listener.Close()
	t.listener = nil

	return err
}

func (t *TCPTransportListener) Accept() (Transport, error) {
	if t.listener == nil {
		return nil, errors.New("listener is not started")
	}

	conn, err := t.listener.Accept()
	if err != nil {
		return nil, err
	}

	transport := TCPTransport{}
	transport.setConn(conn)
	transport.server = true
	transport.TLSConfig = t.TLSConfig
	transport.ReadLimit = t.ReadLimit
	transport.ReadTimeout = t.ReadTimeout
	transport.WriteTimeout = t.WriteTimeout

	return &transport, nil
}
