package lime

import (
	"context"
	"net"
	"time"
)

type InMemoryTransport struct {
	localChan  chan Envelope
	remoteChan chan Envelope
}

func (t *InMemoryTransport) Send(e Envelope) error {
	panic("implement me")
}

func (t *InMemoryTransport) Receive() (Envelope, error) {
	panic("implement me")
}

func (t *InMemoryTransport) Open(ctx context.Context, addr net.Addr) error {
	panic("implement me")
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
