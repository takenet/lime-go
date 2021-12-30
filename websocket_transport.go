package lime

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
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
	conn *websocket.Conn
	c    SessionCompression
	e    SessionEncryption
}

func (t *websocketTransport) Send(ctx context.Context, e Envelope) error {
	if ctx == nil {
		panic("nil context")
	}

	if e == nil || reflect.ValueOf(e).IsNil() {
		panic("nil envelope")
	}

	if err := t.ensureOpen(); err != nil {
		return err
	}

	// Sets the timeout for the next write operation
	deadline, _ := ctx.Deadline()
	if err := t.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}

	return t.conn.WriteJSON(e)
}

func (t *websocketTransport) Receive(ctx context.Context) (Envelope, error) {
	if ctx == nil {
		panic("nil context")
	}

	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	var raw rawEnvelope

	// TODO: Support context
	if err := t.conn.ReadJSON(&raw); err != nil {
		return nil, err
	}

	return raw.ToEnvelope()
}

func (t *websocketTransport) Close() error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	err := t.conn.Close()
	t.conn = nil
	return err
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
	return t.conn != nil
}

func (t *websocketTransport) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}

func (t *websocketTransport) RemoteAddr() net.Addr {
	return t.conn.RemoteAddr()
}

func (t *websocketTransport) ensureOpen() error {
	if t.conn == nil {
		return errors.New("transport is not open")
	}

	return nil
}

type WebsocketConfig struct {
	TLSConfig         *tls.Config
	TraceWriter       TraceWriter // TraceWriter sets the trace writer for tracing connection envelopes
	EnableCompression bool
	ConnBuffer        int
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
	listErr := l.listener.Close()
	if srvErr != nil {
		return srvErr
	}
	if listErr != nil {
		return listErr
	}

	return nil
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
