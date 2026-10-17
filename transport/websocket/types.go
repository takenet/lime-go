package websocket

import (
	"github.com/takenet/lime-go/codec"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/transport"
)

type EnvelopeValue = protocol.EnvelopeValue
type SessionCompression = protocol.SessionCompression
type SessionEncryption = protocol.SessionEncryption
type Transport = transport.Transport
type TransportListener = transport.TransportListener
type TraceWriter = transport.TraceWriter

const (
	SessionCompressionNone = protocol.SessionCompressionNone
	SessionEncryptionNone  = protocol.SessionEncryptionNone
	SessionEncryptionTLS   = protocol.SessionEncryptionTLS
)

func UnmarshalEnvelope(data []byte) (EnvelopeValue, error) {
	return codec.UnmarshalEnvelope(data)
}
