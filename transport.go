package lime

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"time"
)

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

type JSONTransport struct {
	conn    net.Conn
	decoder *json.Decoder
	encoder *json.Encoder
}

func (t *JSONTransport) setConn(conn net.Conn) {
	t.conn = conn

	// TODO: Should we use a buffer here?
	//bufio.NewReaderSize(conn, 100)
	t.encoder = json.NewEncoder(t.conn)
	t.decoder = json.NewDecoder(t.conn)
}

func (t *JSONTransport) ensureOpen() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return nil
}

type TransportListener interface {
	Open(ctx context.Context, addr net.Addr) error

	Close() error

	Accept() (Transport, error)
}
