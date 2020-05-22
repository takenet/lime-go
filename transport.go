package lime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type Transport interface {
	// Sends an envelope to the remote node.
	Send(ctx context.Context, e Envelope) error

	// Receives an envelope from the remote node.
	Receive(ctx context.Context) (Envelope, error)

	// Opens the transport connection with the specified Uri.
	Open(ctx context.Context, addr net.Addr) error

	// Closes the connection.
	Close(ctx context.Context) error

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
	SetEncryption(e SessionEncryption) error

	// Indicates if the transport is connected.
	IsConnected() bool

	// Gets the local endpoint address.
	LocalAdd() net.Addr

	// Gets the remote endpoint address.
	RemoteAdd() net.Addr
}

const DefaultReadLimit int64 = 8192 * 1024
const DefaultWriteTimeout time.Duration = time.Second * 60
const DefaultReadTimeout time.Duration = 0

// Common configurations for net.Conn based transports.
type ConnTransportConfig struct {
	// The limit for buffered data in read operations.
	ReadLimit int64

	// The timeout for read operations
	ReadTimeout time.Duration

	// The timeout for write operations
	WriteTimeout time.Duration

	// The trace writer for tracing connection envelopes
	TraceWriter TraceWriter
}

// Base type for net.Conn based transports.
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
	if err := t.conn.SetWriteDeadline(time.Now().Add(t.WriteTimeout)); err != nil {
		return err
	}
	return t.encoder.Encode(e)
}

func (t *ConnTransport) Receive(context.Context) (Envelope, error) {
	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	// Sets the timeout for the next read operation
	if err := t.conn.SetReadDeadline(time.Now().Add(t.ReadTimeout)); err != nil {
		return nil, err
	}

	// Decode as a map of raw JSON to be unmarshalled
	var m map[string]*json.RawMessage
	if err := t.decoder.Decode(&m); err != nil {
		return nil, err
	}

	// Reset the read limit
	t.limitedReader.N = t.ReadLimit

	return UnmarshalJSONMap(m)
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
		writer = io.MultiWriter(writer, *tw.getSendWriter())
		reader = io.TeeReader(reader, *tw.getReceiveWriter())
	}

	// Sets the decoder to be used for sending envelopes
	t.encoder = json.NewEncoder(writer)

	if t.ReadLimit == 0 {
		t.ReadLimit = DefaultReadLimit
	}

	// Using a LimitedReader to avoid the connection be
	// flooded with a very large JSON which can cause
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

// Defines a listener interface for the transports.
type TransportListener interface {
	// Start listening for new transport connections.
	Open(ctx context.Context, addr net.Addr) error

	// Stop the listener.
	Close() error

	// Accept a new transport connection.
	Accept() (Transport, error)
}

// Enable request tracing for network transports.
type TraceWriter interface {
	// Gets the sendWriter for the transport send operations
	getSendWriter() *io.Writer

	// Gets the sendWriter for the transport receive operations
	getReceiveWriter() *io.Writer
}

// Implements a TraceWriter that uses the standard output for
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

func (t StdoutTraceWriter) getSendWriter() *io.Writer {
	return &t.sendWriter
}

func (t StdoutTraceWriter) getReceiveWriter() *io.Writer {
	return &t.receiveWriter
}
