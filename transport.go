package lime

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

//
//func (e *Envelope) ToEnvelope() interface{} {
//	if e.Message != nil {
//		return *e.Message
//	}
//}
//
//func (e EnvelopeUnion) MarshalJSON() ([]byte, error) {
//
//	return json.Marshal(sw)
//}

//func (e *EnvelopeUnion) UnmarshalJSON(b []byte) error {
//
//	return nil
//}

type Transport interface {
	// Sends an envelope to the remote node.
	Send(e Envelope) error

	// Receives an envelope from the remote node.
	Receive() (Envelope, error)

	// Opens the transport connection with the specified Uri.
	Open(ctx context.Context, addr net.Addr) error

	// Closes the connection.
	Close() error

	// Enumerates the supported compression options for the transport.
	GetSupportedCompression() []SessionCompression

	// Gets the current transport compression option.
	GetCompression() SessionCompression

	// Defines the compression mode for the transport.
	SetCompression(c SessionCompression) error

	// Enumerates the supported encryption options for the transport.
	GetSupportedEncryption() []SessionEncryption

	// Gets the current transport encryption option.
	GetEncryption() SessionEncryption

	// Defines the encryption mode for the transport.
	SetEncryption(c SessionEncryption) error

	// Indicates if the transport is connected.
	OK() bool

	// Gets the local endpoint address.
	LocalAdd() net.Addr

	// Gets the remote endpoint address.
	RemoteAdd() net.Addr

	// Set the transport timeout.
	SetDeadline(time time.Time) error
}

type JsonTransport struct {
	conn    net.Conn
	decoder *json.Decoder
	encoder *json.Encoder
}

func (t *JsonTransport) setConn(conn net.Conn) {
	t.conn = conn
	t.encoder = json.NewEncoder(t.conn)
	t.decoder = json.NewDecoder(t.conn)
}

func (t *JsonTransport) ensureOpen() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return nil
}

type TcpTransport struct {
	JsonTransport
}

func (t *TcpTransport) Send(e Envelope) error {
	err := t.ensureOpen()
	if err != nil {
		return err
	}

	return t.encoder.Encode(e)
}

func (t *TcpTransport) Receive() (Envelope, error) {
	err := t.ensureOpen()
	if err != nil {
		return nil, err
	}

	var m map[string]*json.RawMessage
	err = t.decoder.Decode(&m)
	if err != nil {
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
	panic("implement me")
}

func (t *TcpTransport) GetCompression() SessionCompression {
	panic("implement me")
}

func (t *TcpTransport) SetCompression(c SessionCompression) error {
	panic("implement me")
}

func (t *TcpTransport) GetSupportedEncryption() []SessionEncryption {
	panic("implement me")
}

func (t *TcpTransport) GetEncryption() SessionEncryption {
	panic("implement me")
}

func (t *TcpTransport) SetEncryption(c SessionEncryption) error {
	panic("implement me")
}

func (t *TcpTransport) OK() bool {
	panic("implement me")
}

func (t *TcpTransport) LocalAdd() net.Addr {
	panic("implement me")
}

func (t *TcpTransport) RemoteAdd() net.Addr {
	panic("implement me")
}

func (t *TcpTransport) SetDeadline(time time.Time) error {
	panic("implement me")
}
