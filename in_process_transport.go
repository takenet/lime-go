package lime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

type inProcessTransport struct {
	remote  *inProcessTransport // The remote party
	addr    InProcessAddr
	envChan chan envelope
	done    chan bool
	closed  bool
	mu      sync.RWMutex
}

func (t *inProcessTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.closed {
		t.closed = true
		t.done <- true
	}

	if !t.remote.closed {
		// We are not closing the envChan here to avoid panics on Send method
		return t.remote.Close()
	}

	return nil
}

func (t *inProcessTransport) Send(_ context.Context, e envelope) error {
	if !t.Connected() {
		return errors.New("transport is closed")
	}
	t.remote.envChan <- e
	return nil
}

func (t *inProcessTransport) Receive(ctx context.Context) (envelope, error) {
	if !t.Connected() {
		return nil, errors.New("transport is closed")
	}
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("receive: %w", ctx.Err())
	case <-t.done:
		return nil, errors.New("transport was closed while receiving")
	case e := <-t.envChan:
		return e, nil
	}
}

func newInProcessTransport(addr InProcessAddr, bufferSize int) *inProcessTransport {
	return &inProcessTransport{
		addr:    addr,
		envChan: make(chan envelope, bufferSize),
		done:    make(chan bool, 1),
	}
}

func newInProcessTransportPair(addr InProcessAddr, bufferSize int) (client *inProcessTransport, server *inProcessTransport) {
	server = newInProcessTransport(addr, bufferSize)
	client = newInProcessTransport(addr, bufferSize)
	server.remote = client
	client.remote = server
	return
}

func (t *inProcessTransport) SupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *inProcessTransport) Compression() SessionCompression {
	return SessionCompressionNone
}

func (t *inProcessTransport) SetCompression(context.Context, SessionCompression) error {
	return errors.New("compression is not supported by in process transport")
}

func (t *inProcessTransport) SupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionNone}
}

func (t *inProcessTransport) Encryption() SessionEncryption {
	return SessionEncryptionNone
}

func (t *inProcessTransport) SetEncryption(context.Context, SessionEncryption) error {
	return errors.New("encryption is not supported by in process transport")
}

func (t *inProcessTransport) Connected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed
}

func (t *inProcessTransport) LocalAddr() net.Addr {
	return t.addr
}

func (t *inProcessTransport) RemoteAddr() net.Addr {
	return t.remote.addr
}

type inProcessTransportListener struct {
	addr       InProcessAddr
	transports chan *inProcessTransport
	done       chan bool
	closed     bool
	closedMu   sync.RWMutex
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
	l.closedMu.Lock()
	defer l.closedMu.Unlock()

	inProcListenersMu.Lock()
	delete(inProcListeners, l.addr)
	inProcListenersMu.Unlock()

	l.closed = true
	l.done <- true
	return nil
}

func (l *inProcessTransportListener) Listen(_ context.Context, addr net.Addr) error {
	inProcAddr, ok := addr.(InProcessAddr)
	if !ok {
		return fmt.Errorf("invalid in process address %s", addr)
	}

	if inProcAddr == "" {
		return fmt.Errorf("empty in process address %s", inProcAddr)
	}

	inProcListenersMu.Lock()
	defer inProcListenersMu.Unlock()

	if _, ok := inProcListeners[inProcAddr]; ok {
		return fmt.Errorf("a listerer is already active on address %s", inProcAddr)
	}
	l.addr = inProcAddr
	inProcListeners[inProcAddr] = l
	return nil
}

func (l *inProcessTransportListener) Accept(ctx context.Context) (Transport, error) {
	if !l.listening() {
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

func (l *inProcessTransportListener) listening() bool {
	l.closedMu.RLock()
	defer l.closedMu.RUnlock()

	return !l.closed
}

func (l *inProcessTransportListener) newClient(addr InProcessAddr, bufferSize int) *inProcessTransport {
	// Create transport pair
	client, server := newInProcessTransportPair(addr, bufferSize)
	go func() {
		l.transports <- server
	}()
	return client
}

var inProcListeners = make(map[InProcessAddr]*inProcessTransportListener)
var inProcListenersMu sync.RWMutex

// DialInProcess creates a new in process transport connection to the specified path.
func DialInProcess(addr InProcessAddr, bufferSize int) (Transport, error) {
	inProcListenersMu.RLock()
	l := inProcListeners[addr]
	inProcListenersMu.RUnlock()

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
