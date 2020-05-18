package lime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

type TCPTransport struct {
	JSONTransport
	encryption SessionEncryption
	server     bool
	TLSConfig  *tls.Config
}

func (t *TCPTransport) Send(e Envelope) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	return t.encoder.Encode(e)
}

func (t *TCPTransport) Receive() (Envelope, error) {
	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	var m map[string]*json.RawMessage

	if err := t.decoder.Decode(&m); err != nil {
		return nil, err
	}

	return UnmarshalJSONMap(m)
}

func (t *TCPTransport) Open(ctx context.Context, addr net.Addr) error {
	if t.conn != nil {
		return errors.New("transport already open")
	}

	if addr.Network() != "tcp" {
		return errors.New("network should be tcp")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, addr.Network(), addr.String())
	if err != nil {
		return err
	}

	t.setConn(conn)
	return nil
}

func (t *TCPTransport) Close() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return t.conn.Close()
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
	// We convert existing connection to TLS
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	t.setConn(tlsConn)
	t.encryption = SessionEncryptionTLS
	return nil
}

func (t *TCPTransport) OK() bool {
	return t.conn != nil
}

func (t *TCPTransport) LocalAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

func (t *TCPTransport) RemoteAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.RemoteAddr()
}

func (t *TCPTransport) SetDeadline(time time.Time) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}
	return t.conn.SetDeadline(time)
}

type TCPTransportListener struct {
	listener net.Listener
	mux      sync.Mutex
}

func (t *TCPTransportListener) Open(ctx context.Context, addr net.Addr) error {
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

	return &transport, nil
}
