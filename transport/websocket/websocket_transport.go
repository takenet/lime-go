// Package websocket provides WebSocket transports and listeners. Transports
// serialize concurrent sends and receives independently. Canceling active I/O
// or encountering an I/O error closes the transport in both directions; calls
// canceled before acquiring their I/O slot leave the connection usable.
package websocket

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/takenet/lime-go/codec"
)

const (
	errWSTransportSend = "ws transport: send: %w"
)

func DialWebsocket(ctx context.Context, urlStr string, requestHeader http.Header, tls *tls.Config) (Transport, error) {
	d := websocket.Dialer{
		TLSClientConfig: tls,
	}

	if requestHeader == nil {
		requestHeader = http.Header{}
	}
	requestHeader["Sec-WebSocket-Protocol"] = []string{"lime"}

	conn, _, err := d.DialContext(ctx, urlStr, requestHeader)
	if err != nil {
		return nil, err
	}

	t := &websocketTransport{conn: conn, c: SessionCompressionNone}
	if strings.HasPrefix(urlStr, "wss:") {
		t.e = SessionEncryptionTLS
	} else {
		t.e = SessionEncryptionNone
	}

	return t, nil
}

type websocketTransport struct {
	conn      *websocket.Conn
	c         SessionCompression
	e         SessionEncryption
	closed    atomic.Bool
	initOnce  sync.Once
	readGate  chan struct{}
	writeGate chan struct{}
	done      chan struct{}
}

func (t *websocketTransport) Send(ctx context.Context, e EnvelopeValue) error {
	if ctx == nil {
		panic("nil context")
	}

	if e == nil || reflect.ValueOf(e).IsNil() {
		panic("nil envelope")
	}

	if err := t.acquire(ctx, false); err != nil {
		return err
	}
	defer func() { <-t.writeGate }()

	payload, err := codec.MarshalEnvelope(e)
	if err != nil {
		return fmt.Errorf("ws transport: marshal envelope: %w", err)
	}
	if err := t.runIO(ctx, func() error {
		return t.conn.WriteMessage(websocket.TextMessage, payload)
	}); err != nil {
		return fmt.Errorf(errWSTransportSend, err)
	}
	return nil
}

func (t *websocketTransport) Receive(ctx context.Context) (EnvelopeValue, error) {
	if ctx == nil {
		panic("nil context")
	}

	if err := t.acquire(ctx, true); err != nil {
		return nil, err
	}
	defer func() { <-t.readGate }()

	var raw json.RawMessage
	if err := t.runIO(ctx, func() error {
		return t.conn.ReadJSON(&raw)
	}); err != nil {
		return nil, fmt.Errorf("ws transport: receive: %w", err)
	}
	return UnmarshalEnvelope(raw)
}

func (t *websocketTransport) initIO() {
	t.initOnce.Do(func() {
		t.readGate = make(chan struct{}, 1)
		t.writeGate = make(chan struct{}, 1)
		t.done = make(chan struct{})
	})
}

func (t *websocketTransport) acquire(ctx context.Context, read bool) error {
	t.initIO()
	gate := t.writeGate
	if read {
		gate = t.readGate
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return t.ensureOpen()
	case gate <- struct{}{}:
	}
	if err := ctx.Err(); err != nil {
		<-gate
		return err
	}
	if err := t.ensureOpen(); err != nil {
		<-gate
		return err
	}
	return nil
}

func (t *websocketTransport) runIO(ctx context.Context, operation func() error) error {
	result := make(chan error, 1)
	go func() { result <- operation() }()
	select {
	case <-ctx.Done():
		_ = t.Close()
		<-result
		return ctx.Err()
	case err := <-result:
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = t.Close()
			return ctxErr
		}
		if err != nil {
			_ = t.Close()
		}
		return err
	}
}

func (t *websocketTransport) Close() error {
	t.initIO()
	if t.conn == nil || !t.closed.CompareAndSwap(false, true) {
		return errors.New("transport is not open")
	}
	close(t.done)
	return t.conn.Close()
}

func (t *websocketTransport) SupportedCompression() []SessionCompression {
	return []SessionCompression{t.c}
}

func (t *websocketTransport) Compression() SessionCompression {
	return t.c
}

func (t *websocketTransport) SetCompression(_ context.Context, c SessionCompression) error {
	if c != t.c {
		return errors.New("compression cannot be changed")
	}
	return nil
}

func (t *websocketTransport) SupportedEncryption() []SessionEncryption {
	return []SessionEncryption{t.e}
}

func (t *websocketTransport) Encryption() SessionEncryption {
	return t.e
}

func (t *websocketTransport) SetEncryption(_ context.Context, e SessionEncryption) error {
	if e != t.e {
		return errors.New("encryption cannot be changed")
	}
	return nil
}

func (t *websocketTransport) Connected() bool {
	return t.conn != nil && !t.closed.Load()
}

func (t *websocketTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

func (t *websocketTransport) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

func (t *websocketTransport) ensureOpen() error {
	if !t.Connected() {
		return errors.New("transport is not open")
	}

	return nil
}

type WebsocketConfig struct {
	TLSConfig         *tls.Config
	TraceWriter       TraceWriter // TraceWriter sets the trace writer for tracing connection envelopes
	EnableCompression bool
	ConnBuffer        int

	// CheckOrigin returns true if the request Origin header is acceptable. If
	// CheckOrigin is nil, then a safe default is used: return false if the
	// Origin request header is present and the origin host is not equal to
	// request Host header.
	//
	// A CheckOrigin function should carefully validate the request origin to
	// prevent cross-site request forgery.
	CheckOrigin func(r *http.Request) bool
}

type websocketTransportListener struct {
	WebsocketConfig
	listener net.Listener
	srv      *http.Server
	upgrader *websocket.Upgrader
	connChan chan *websocket.Conn
	done     chan struct{}
	mu       sync.RWMutex
}

func NewWebsocketTransportListener(config *WebsocketConfig) TransportListener {
	if config == nil {
		config = &WebsocketConfig{}
	}
	return &websocketTransportListener{WebsocketConfig: *config}
}

func (l *websocketTransportListener) Listen(ctx context.Context, addr net.Addr) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.srv != nil {
		return errors.New("ws listener already started")
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", addr.String())
	if err != nil {
		return err
	}
	l.listener = listener
	srv := &http.Server{
		Addr:      addr.String(),
		Handler:   l,
		TLSConfig: l.TLSConfig,
	}
	l.srv = srv
	l.upgrader = &websocket.Upgrader{
		Subprotocols:      []string{"lime"},
		EnableCompression: l.EnableCompression,
		CheckOrigin:       l.CheckOrigin,
	}
	l.connChan = make(chan *websocket.Conn, l.ConnBuffer)
	l.done = make(chan struct{})
	go func() {
		if l.tls() {
			if err := srv.ServeTLS(listener, "", ""); err != nil && err != net.ErrClosed {
				log.Printf("ws listen: %v", err)
			}
		} else {
			if err := srv.Serve(listener); err != nil && err != net.ErrClosed {
				log.Printf("ws listen: %v", err)
			}
		}
	}()

	return nil
}

func (l *websocketTransportListener) tls() bool {
	return l.TLSConfig != nil
}

func (l *websocketTransportListener) Accept(ctx context.Context) (Transport, error) {
	if err := l.ensureStarted(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ws listener: %w", ctx.Err())
	case <-l.done:
		return nil, errors.New("ws listener closed")
	case conn := <-l.connChan:
		ws := &websocketTransport{
			conn: conn,
			c:    SessionCompressionNone,
		}
		if l.tls() {
			ws.e = SessionEncryptionTLS
		} else {
			ws.e = SessionEncryptionNone
		}

		return ws, nil
	}
}

func (l *websocketTransportListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.srv == nil {
		return errors.New("ws listener: listener is not started")
	}

	close(l.done)
	srvErr := l.srv.Close()
	l.srv = nil

	return srvErr
}

func (l *websocketTransportListener) ensureStarted() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.srv == nil {
		return errors.New("ws listener: listener is not started")
	}

	return nil
}

func (l *websocketTransportListener) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	conn, err := l.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Printf("ws listener: serveHTTP: %v\n", err)
		return
	}

	select {
	case <-l.done:
	case l.connChan <- conn:
	}
}
