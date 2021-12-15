package lime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// Transport defines the basic features for a Lime communication mean
type Transport interface {
	io.Closer
	Send(ctx context.Context, e Envelope) error                     // Send sends an envelope to the remote node.
	Receive(ctx context.Context) (Envelope, error)                  // Receive receives an envelope from the remote node.
	GetSupportedCompression() []SessionCompression                  // GetSupportedCompression enumerates the supported compression options for the transport.
	GetCompression() SessionCompression                             // GetCompression gets the current transport compression option.
	SetCompression(ctx context.Context, c SessionCompression) error // SetCompression defines the compression mode for the transport.
	GetSupportedEncryption() []SessionEncryption                    // GetSupportedEncryption enumerates the supported encryption options for the transport.
	GetEncryption() SessionEncryption                               // GetEncryption gets the current transport encryption option.
	SetEncryption(ctx context.Context, e SessionEncryption) error   // SetEncryption defines the encryption mode for the transport.
	Connected() bool                                                // Connected indicates if the transport is connected.
	LocalAddr() net.Addr                                            // LocalAddr gets the local endpoint address.
	RemoteAddr() net.Addr                                           // RemoteAddr gets the remote endpoint address.
}

// TransportListener Defines a listener interface for the transports.
type TransportListener interface {
	io.Closer
	Listen(ctx context.Context, addr net.Addr) error // Listen start listening for new transport connections.
	Accept(ctx context.Context) (Transport, error)   // Accept a new transport connection.
}

// TraceWriter Enable request tracing for network transports.
type TraceWriter interface {
	SendWriter() *io.Writer    // SendWriter returns the sendWriter for the transport send operations
	ReceiveWriter() *io.Writer // ReceiveWriter returns the sendWriter for the transport receive operations
}

// StdoutTraceWriter Implements a TraceWriter that uses the standard output for
// writing send and received envelopes.
type StdoutTraceWriter struct {
	sendWriter    io.Writer
	receiveWriter io.Writer
}

func NewStdoutTraceWriter() TraceWriter {
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
