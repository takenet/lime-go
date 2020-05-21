package lime

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// Implements an InMemory transport type
type InMemoryTransport struct {
	BufferSize int
	localChan  chan Envelope
	server     bool
	addr       net.Addr
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
	if t.localChan != nil {
		return errors.New("transport already open")
	}

	if addr.Network() != InProcessNetwork {
		return fmt.Errorf("address network should be %v", InProcessNetwork)
	}

	t.localChan = make(chan Envelope, t.BufferSize)
	t.addr = addr
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
	return nil
}

func (t *InMemoryTransport) IsConnected() bool {
	return t.localChan != nil
}

func (t *InMemoryTransport) LocalAdd() net.Addr {
	return t.addr
}

func (t *InMemoryTransport) RemoteAdd() net.Addr {
	return t.remote.addr
}

const InProcessNetwork = "in.process"

type InProcessAddr string

func (i InProcessAddr) Network() string {
	return InProcessNetwork
}

func (i InProcessAddr) String() string {
	return string(i)
}
