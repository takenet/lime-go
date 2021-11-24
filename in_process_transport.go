package lime

import (
	"context"
	"errors"
	"fmt"
	"net"
)

type inProcessTransport struct {
	remote  *inProcessTransport // The remote party
	addr    InProcessAddr
	envChan chan Envelope
	done    chan bool
	closed  bool
}

func (t *inProcessTransport) Close() error {
	if !t.closed {
		t.closed = true
		t.done <- true
		// We are not closing the envChan here to avoid panics on Send method
		return t.remote.Close()
	}
	return nil
}

func (t *inProcessTransport) Send(_ context.Context, e Envelope) error {
	if t.closed {
		return errors.New("transport is closed")
	}
	t.remote.envChan <- e
	return nil
}

func (t *inProcessTransport) Receive(ctx context.Context) (Envelope, error) {
	if t.closed {
		return nil, errors.New("transport is closed")
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, errors.New("transport was closed while receiving")
	case e := <-t.envChan:
		return e, nil
	}
}

func newInProcessTransport(addr InProcessAddr, bufferSize int) *inProcessTransport {
	return &inProcessTransport{
		addr:    addr,
		envChan: make(chan Envelope, bufferSize),
		done:    make(chan bool, 1),
	}
}

func (t *inProcessTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *inProcessTransport) GetCompression() SessionCompression {
	return SessionCompressionNone
}

func (t *inProcessTransport) SetCompression(context.Context, SessionCompression) error {
	return errors.New("compression is not supported by in process transport")
}

func (t *inProcessTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionNone}
}

func (t *inProcessTransport) GetEncryption() SessionEncryption {
	return SessionEncryptionNone
}

func (t *inProcessTransport) SetEncryption(context.Context, SessionEncryption) error {
	return errors.New("encryption is not supported by in process transport")
}

func (t *inProcessTransport) IsConnected() bool {
	return !t.closed
}

func (t *inProcessTransport) LocalAdd() net.Addr {

	panic("implement me")
}

func (t *inProcessTransport) RemoteAdd() net.Addr {
	panic("implement me")
}

type inProcessTransportListener struct {
	addr       InProcessAddr
	transports chan *inProcessTransport
	done       chan bool
}

func NewInProcessTransportListener(addr InProcessAddr) TransportListener {
	l := &inProcessTransportListener{
		addr:       addr,
		transports: make(chan *inProcessTransport, 1),
		done:       make(chan bool, 1),
	}
	return l
}

func (l *inProcessTransportListener) Close() error {
	delete(listeners, l.addr)
	l.addr = ""
	l.done <- true
	return nil
}

func (l *inProcessTransportListener) Listen(ctx context.Context, addr net.Addr) error {
	inProcAddr, ok := addr.(InProcessAddr)
	if !ok {
		return fmt.Errorf("invalid in process address %s", addr)
	}

	if inProcAddr == "" {
		return fmt.Errorf("empty in process address %s", inProcAddr)
	}

	if _, ok := listeners[inProcAddr]; ok {
		return fmt.Errorf("a listerer is already active on address %s", inProcAddr)
	}
	l.addr = inProcAddr
	listeners[inProcAddr] = l
	return nil
}

func (l *inProcessTransportListener) Accept(ctx context.Context) (Transport, error) {
	if l.addr == "" {
		return nil, errors.New("listener is not active")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-l.done:
		return nil, errors.New("listener stopped")
	case t := <-l.transports:
		return t, nil
	}
}

func (l *inProcessTransportListener) newClient(addr InProcessAddr, bufferSize int) *inProcessTransport {
	// Create transport pair
	serverTransport := newInProcessTransport(addr, bufferSize)
	clientTransport := newInProcessTransport(addr, bufferSize)
	serverTransport.remote = clientTransport
	clientTransport.remote = serverTransport
	go func() {
		l.transports <- serverTransport
	}()

	return clientTransport
}

var listeners = make(map[InProcessAddr]*inProcessTransportListener)

// DialInProcess creates a new in process transport connection to the specified path.
func DialInProcess(addr InProcessAddr, bufferSize int) (*inProcessTransport, error) {
	l := listeners[addr]
	if l == nil {
		return nil, fmt.Errorf("in process connection refused on %s address", addr)
	}

	return l.newClient(addr, bufferSize), nil
}

const InProcessNetwork = "in.process"

type InProcessAddr string

func (i InProcessAddr) Network() string {
	return InProcessNetwork
}

func (i InProcessAddr) String() string {
	return string(i)
}
