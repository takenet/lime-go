package lime

import (
	"context"
	"errors"
	"net"
	"time"
)

type InMemoryTransport struct {
	BufferSize int
	localChan  chan Envelope
	server     bool
	remote     *InMemoryTransport
}

func NewInMemoryTransportPair(bufferSize int) (client InMemoryTransport, server InMemoryTransport) {
	client = InMemoryTransport{BufferSize: bufferSize, server: false}
	server = InMemoryTransport{BufferSize: bufferSize, server: true}
	client.remote = &server
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
	panic("implement me")
}

func (t *InMemoryTransport) GetSupportedCompression() []SessionCompression {
	panic("implement me")
}

func (t *InMemoryTransport) GetCompression() SessionCompression {
	panic("implement me")
}

func (t *InMemoryTransport) SetCompression(c SessionCompression) error {
	panic("implement me")
}

func (t *InMemoryTransport) GetSupportedEncryption() []SessionEncryption {
	panic("implement me")
}

func (t *InMemoryTransport) GetEncryption() SessionEncryption {
	panic("implement me")
}

func (t *InMemoryTransport) SetEncryption(c SessionEncryption) error {
	panic("implement me")
}

func (t *InMemoryTransport) OK() bool {
	panic("implement me")
}

func (t *InMemoryTransport) LocalAdd() net.Addr {
	panic("implement me")
}

func (t *InMemoryTransport) RemoteAdd() net.Addr {
	panic("implement me")
}

func (t *InMemoryTransport) SetDeadline(time time.Time) error {
	panic("implement me")
}
