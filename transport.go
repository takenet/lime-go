package lime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
)

// Transport defines the basic features for a Lime communication mean
type Transport interface {
	io.Closer
	Send(ctx context.Context, e envelope) error                     // Send sends an envelope to the remote node.
	Receive(ctx context.Context) (envelope, error)                  // Receive receives an envelope from the remote node.
	SupportedCompression() []SessionCompression                     // SupportedCompression enumerates the supported compression options for the transport.
	Compression() SessionCompression                                // Compression returns the current transport compression option.
	SetCompression(ctx context.Context, c SessionCompression) error // SetCompression defines the compression mode for the transport.
	SupportedEncryption() []SessionEncryption                       // SupportedEncryption enumerates the supported encryption options for the transport.
	Encryption() SessionEncryption                                  // Encryption returns the current transport encryption option.
	SetEncryption(ctx context.Context, e SessionEncryption) error   // SetEncryption defines the encryption mode for the transport.
	Connected() bool                                                // Connected indicates if the transport is connected.
	LocalAddr() net.Addr                                            // LocalAddr returns the local endpoint address.
	RemoteAddr() net.Addr                                           // RemoteAddr returns the remote endpoint address.
}

// TransportListener Defines a listener interface for the transports.
type TransportListener interface {
	io.Closer
	Listen(ctx context.Context, addr net.Addr) error // Listen start listening for new transport connections.
	Accept(ctx context.Context) (Transport, error)   // Accept a new transport connection.
}

// TraceWriter Enable request tracing for network transports.
type TraceWriter interface {
	io.Closer
	SendWriter() *io.Writer    // SendWriter returns the sendWriter for the transport send operations
	ReceiveWriter() *io.Writer // ReceiveWriter returns the sendWriter for the transport receive operations
}

// StdoutTraceWriter Implements a TraceWriter that uses the standard output for
// writing send and received envelopes.
type StdoutTraceWriter struct {
	sendWriter    io.WriteCloser
	receiveWriter io.WriteCloser
	sendReader    io.ReadCloser
	receiveReader io.ReadCloser
}

func NewStdoutTraceWriter() TraceWriter {
	sendReader, sendWriter := io.Pipe()
	receiveReader, receiveWriter := io.Pipe()
	sendDecoder := json.NewDecoder(sendReader)
	receiveDecoder := json.NewDecoder(receiveReader)

	tw := &StdoutTraceWriter{
		sendWriter:    sendWriter,
		receiveWriter: receiveWriter,
		sendReader:    sendReader,
		receiveReader: receiveReader,
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

	return tw
}

func (t *StdoutTraceWriter) SendWriter() *io.Writer {
	w := io.Writer(t.sendWriter)
	return &w
}

func (t *StdoutTraceWriter) ReceiveWriter() *io.Writer {
	w := io.Writer(t.receiveWriter)
	return &w
}

func (t *StdoutTraceWriter) Close() error {
	// Close writers first to signal goroutines to exit
	// Accumulate all errors using errors.Join to ensure all resources are closed
	sendWriterErr := t.sendWriter.Close()
	receiveWriterErr := t.receiveWriter.Close()
	// Close readers to clean up resources
	sendReaderErr := t.sendReader.Close()
	receiveReaderErr := t.receiveReader.Close()

	return errors.Join(sendWriterErr, receiveWriterErr, sendReaderErr, receiveReaderErr)
}
