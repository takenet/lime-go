package lime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
)

type Transport interface {
	// Send sends an envelope to the remote node.
	Send(ctx context.Context, e Envelope) error

	// Receive receives an envelope from the remote node.
	Receive(ctx context.Context) (Envelope, error)

	// Open opens the transport connection with the specified Uri.
	Open(ctx context.Context, addr net.Addr) error

	// Close closes the connection.
	Close(ctx context.Context) error

	// GetSupportedCompression enumerates the supported compression options for the transport.
	GetSupportedCompression() []SessionCompression

	// GetCompression gets the current transport compression option.
	GetCompression() SessionCompression

	// SetCompression defines the compression mode for the transport.
	SetCompression(ctx context.Context, c SessionCompression) error

	// GetSupportedEncryption enumerates the supported encryption options for the transport.
	GetSupportedEncryption() []SessionEncryption

	// GetEncryption gets the current transport encryption option.
	GetEncryption() SessionEncryption

	// SetEncryption defines the encryption mode for the transport.
	SetEncryption(ctx context.Context, e SessionEncryption) error

	// IsConnected indicates if the transport is connected.
	IsConnected() bool

	// LocalAdd gets the local endpoint address.
	LocalAdd() net.Addr

	// RemoteAdd gets the remote endpoint address.
	RemoteAdd() net.Addr
}

const DefaultReadLimit int64 = 8192 * 1024

// ConnTransportConfig defines the common configurations for net.Conn based transports.
type ConnTransportConfig struct {
	// The limit for buffered data in read operations.
	ReadLimit int64

	// The trace writer for tracing connection envelopes
	TraceWriter TraceWriter
}

// ConnTransport implement a base type for net.Conn based transports.
type ConnTransport struct {
	ConnTransportConfig
	conn          net.Conn
	encoder       *json.Encoder
	decoder       *json.Decoder
	limitedReader io.LimitedReader
}

func (t *ConnTransport) Send(ctx context.Context, e Envelope) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	// Sets the timeout for the next write operation
	deadline, _ := ctx.Deadline()
	if err := t.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	// TODO: Encode writes a new line after each entry, how we can avoid this?
	return t.encoder.Encode(e)
}

func (t *ConnTransport) Receive(ctx context.Context) (Envelope, error) {
	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	// Sets the timeout for the next read operation
	deadline, _ := ctx.Deadline()
	if err := t.conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	var raw RawEnvelope

	if err := t.decoder.Decode(&raw); err != nil {
		return nil, err
	}

	// Reset the read limit
	t.limitedReader.N = t.ReadLimit

	return raw.ToEnvelope()
}

func (t *ConnTransport) Close(context.Context) error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	err := t.conn.Close()
	t.conn = nil
	return err
}

func (t *ConnTransport) IsConnected() bool {
	return t.conn != nil
}

func (t *ConnTransport) LocalAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

func (t *ConnTransport) RemoteAdd() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.RemoteAddr()
}

func (t *ConnTransport) setConn(conn net.Conn) {
	t.conn = conn

	var writer io.Writer = t.conn
	var reader io.Reader = t.conn

	// Configure the trace writer, if defined
	tw := t.TraceWriter
	if tw != nil {
		writer = io.MultiWriter(writer, *tw.SendWriter())
		reader = io.TeeReader(reader, *tw.ReceiveWriter())
	}

	// Sets the encoder to be used for sending envelopes
	t.encoder = json.NewEncoder(writer)

	if t.ReadLimit == 0 {
		t.ReadLimit = DefaultReadLimit
	}

	// Using a LimitedReader to avoid the connection be
	// flooded with a very large JSON which will cause a
	// high memory usage.
	t.limitedReader = io.LimitedReader{
		R: reader,
		N: t.ReadLimit,
	}
	t.decoder = json.NewDecoder(&t.limitedReader)
}

func (t *ConnTransport) ensureOpen() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return nil
}

// TransportListener Defines a listener interface for the transports.
type TransportListener interface {
	// Open Start listening for new transport connections.
	Open(ctx context.Context, addr net.Addr) error

	// Close Stop the listener.
	Close() error

	// Accept a new transport connection.
	Accept() (Transport, error)
}

// TraceWriter Enable request tracing for network transports.
type TraceWriter interface {
	// SendWriter returns the sendWriter for the transport send operations
	SendWriter() *io.Writer

	// ReceiveWriter returns the sendWriter for the transport receive operations
	ReceiveWriter() *io.Writer
}

// StdoutTraceWriter Implements a TraceWriter that uses the standard output for
// writing send and received envelopes.
type StdoutTraceWriter struct {
	sendWriter    io.Writer
	receiveWriter io.Writer
}

func NewStdoutTraceWriter() *StdoutTraceWriter {
	sendReader, sendWriter := io.Pipe()
	receiveReader, receiveWriter := io.Pipe()
	sendDecoder := json.NewDecoder(sendReader)
	receiveDecoder := json.NewDecoder(receiveReader)

	tw := StdoutTraceWriter{
		sendWriter:    sendWriter,
		receiveWriter: receiveWriter,
	}
	trace := func(dec *json.Decoder, action string) {
		for {
			var j json.RawMessage
			err := dec.Decode(&j)
			if err != nil {
				break
			}

			fmt.Printf("%v: %v\n", action, string(j))
		}
	}

	go trace(receiveDecoder, "receive")
	go trace(sendDecoder, "send")

	return &tw
}

func (t StdoutTraceWriter) SendWriter() *io.Writer {
	return &t.sendWriter
}

func (t StdoutTraceWriter) ReceiveWriter() *io.Writer {
	return &t.receiveWriter
}
