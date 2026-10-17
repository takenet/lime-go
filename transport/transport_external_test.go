package transport_test

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/transport"
)

type externalTransport struct{}

func (externalTransport) Close() error                                            { return nil }
func (externalTransport) Send(context.Context, protocol.EnvelopeValue) error      { return nil }
func (externalTransport) Receive(context.Context) (protocol.EnvelopeValue, error) { return nil, io.EOF }
func (externalTransport) SupportedCompression() []protocol.SessionCompression     { return nil }
func (externalTransport) Compression() protocol.SessionCompression {
	return protocol.SessionCompressionNone
}
func (externalTransport) SetCompression(context.Context, protocol.SessionCompression) error {
	return nil
}
func (externalTransport) SupportedEncryption() []protocol.SessionEncryption { return nil }
func (externalTransport) Encryption() protocol.SessionEncryption {
	return protocol.SessionEncryptionNone
}
func (externalTransport) SetEncryption(context.Context, protocol.SessionEncryption) error { return nil }
func (externalTransport) Connected() bool                                                 { return true }
func (externalTransport) LocalAddr() net.Addr                                             { return nil }
func (externalTransport) RemoteAddr() net.Addr                                            { return nil }

func TestTransportCanBeImplementedOutsidePackage(t *testing.T) {
	var implementation transport.Transport = externalTransport{}
	require.True(t, implementation.Connected())
}
