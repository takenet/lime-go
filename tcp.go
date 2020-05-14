package lime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type TcpTransport struct {
	JsonTransport
	encryption SessionEncryption
	server     bool
	TlsConfig  *tls.Config
}

func (t *TcpTransport) Send(e Envelope) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	return t.encoder.Encode(e)
}

func (t *TcpTransport) Receive() (Envelope, error) {
	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	var m map[string]*json.RawMessage

	if err := t.decoder.Decode(&m); err != nil {
		return nil, err
	}

	return UnmarshalJSONMap(m)
}

func (t *TcpTransport) Open(ctx context.Context, addr net.Addr) error {
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

func (t *TcpTransport) Close() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return t.conn.Close()
}

func (t *TcpTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *TcpTransport) GetCompression() SessionCompression {
	return SessionCompressionNone
}

func (t *TcpTransport) SetCompression(c SessionCompression) error {
	return fmt.Errorf("compression '%v' is not supported", c)
}

func (t *TcpTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}
}

func (t *TcpTransport) GetEncryption() SessionEncryption {
	return t.encryption
}

func (t *TcpTransport) SetEncryption(e SessionEncryption) error {
	if e == t.encryption {
		return nil
	}

	if e == SessionEncryptionNone {
		return errors.New("cannot downgrade from tls to none encryption")
	}

	if e == SessionEncryptionTLS && t.TlsConfig == nil {
		return errors.New("tls config must be defined")
	}

	var tlsConn *tls.Conn

	// https://github.com/FluuxIO/go-xmpp/blob/master/xmpp_transport.go#L80
	if t.server {
		tlsConn = tls.Server(t.conn, t.TlsConfig)
	} else {
		tlsConn = tls.Client(t.conn, t.TlsConfig)
	}
	// We convert existing connection to TLS
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	t.setConn(tlsConn)
	t.encryption = SessionEncryptionTLS
	return nil
}

func (t *TcpTransport) OK() bool {
	return t.conn != nil
}

func (t *TcpTransport) LocalAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

func (t *TcpTransport) RemoteAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.RemoteAddr()
}

func (t *TcpTransport) SetDeadline(time time.Time) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}
	return t.conn.SetDeadline(time)
}
