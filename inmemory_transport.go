package lime

import (
	"context"
	"errors"
	"net"
	"time"
)

// Implements an InMemory transport type
type InMemoryTransport struct {
	BufferSize int
	localChan  chan Envelope
	server     bool
	remote     *InMemoryTransport
}

func NewInMemoryTransportPair(bufferSize int) (client *InMemoryTransport, server *InMemoryTransport) {
	client = &InMemoryTransport{BufferSize: bufferSize, server: false}
	server = &InMemoryTransport{BufferSize: bufferSize, server: true}
	client.remote = server
	return
}

func (t *InMemoryTransport) Send(e Envelope) error {
	if t.remote.localChan == nil {
		return errors.New("remote transport is not open")
	}

	t.remote.localChan <- e
	return nil
}

func (t *InMemoryTransport) Receive() (Envelope, error) {
	if t.localChan == nil {
		return nil, errors.New("transport is not open")
	}
	e, ok := <-t.localChan
	if !ok {
		return nil, errors.New("the transport is closed")
	}
	return e, nil
}

func (t *InMemoryTransport) Open(ctx context.Context, addr net.Addr) error {
	t.localChan = make(chan Envelope, t.BufferSize)
	return nil
}

func (t *InMemoryTransport) Close() error {
	close(t.localChan)
	t.localChan = nil
	return nil
}

func (t *InMemoryTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *InMemoryTransport) GetCompression() SessionCompression {
	return SessionCompressionNone
}

func (t *InMemoryTransport) SetCompression(c SessionCompression) error {
	if c != SessionCompressionNone {
		return errors.New("unsupported compression")
	}
	return nil
}

func (t *InMemoryTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionTLS}
}

func (t *InMemoryTransport) GetEncryption() SessionEncryption {
	return SessionEncryptionTLS
}

func (t *InMemoryTransport) SetEncryption(e SessionEncryption) error {
	if e != SessionEncryptionNone {
		return errors.New("unsupported encryption")
	}
}

func (t *InMemoryTransport) OK() bool {
	return t.localChan != nil
}

func (t *InMemoryTransport) LocalAdd() net.Addr {
	return net.Addr("")
}

func (t *InMemoryTransport) RemoteAdd() net.Addr {
	panic("implement me")
}

func (t *InMemoryTransport) SetDeadline(time time.Time) error {
	panic("implement me")
}

type InProcessAddr string

func (i InProcessAddr) Network() string {
	return "in.process"
}

func (i InProcessAddr) String() string {
	return string(i)
}
