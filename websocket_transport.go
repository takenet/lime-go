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

	var raw RawEnvelope

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

	if err := t.conn.Close(); err != nil {
		return err
	}

	t.conn = nil
	return nil
}

func (t *websocketTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{t.c}
}

func (t *websocketTransport) GetCompression() SessionCompression {
	return t.c
}

func (t *websocketTransport) SetCompression(ctx context.Context, c SessionCompression) error {
	if c != t.c {
		return errors.New("compression cannot be changed")
	}
	return nil
}

func (t *websocketTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{t.e}
}

func (t *websocketTransport) GetEncryption() SessionEncryption {
	return t.e
}

func (t *websocketTransport) SetEncryption(ctx context.Context, e SessionEncryption) error {
	if e != t.e {
		return errors.New("encryption cannot be changed")
	}
	return nil
}

func (t *websocketTransport) IsConnected() bool {
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

type WebsocketTransportListener struct {
	CertFile          string // CertFile
	KeyFile           string
	TLSConfig         *tls.Config
	EnableCompression bool
	listener          net.Listener
	srv               *http.Server
	upgrader          *websocket.Upgrader
	transportBuffer   int
	transportChan     chan *websocketTransport
	done              chan bool
	errChan           chan error
	mu                sync.Mutex
}

func (l *WebsocketTransportListener) Listen(ctx context.Context, addr net.Addr) error {
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
	l.transportChan = make(chan *websocketTransport, l.transportBuffer)
	l.done = make(chan bool)
	l.errChan = make(chan error)

	go func() {
		if l.tls() {
			if err := srv.ServeTLS(listener, l.CertFile, l.KeyFile); err != nil {
				l.errChan <- fmt.Errorf("ws listener: %w", err)
			}
		} else {
			if err := srv.Serve(listener); err != nil {
				l.errChan <- fmt.Errorf("ws listener: %w", err)
			}
		}
		_ = l.Close()
	}()

	return nil
}

func (l *WebsocketTransportListener) tls() bool {
	return l.TLSConfig != nil || (l.CertFile != "" && l.KeyFile != "")
}

func (l *WebsocketTransportListener) Accept(ctx context.Context) (Transport, error) {
	if err := l.ensureStarted(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ws listener: %w", ctx.Err())
	case <-l.done:
		return nil, errors.New("ws listener closed")
	case err := <-l.errChan:
		return nil, err
	case transport, ok := <-l.transportChan:
		if !ok {
			return nil, errors.New("ws listener closed")
		}
		return transport, nil
	}
}

func (l *WebsocketTransportListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureStarted(); err != nil {
		return err
	}

	if err := l.srv.Close(); err != nil {
		return err
	}

	if err := l.listener.Close(); err != nil {
		return err
	}

	l.srv = nil
	close(l.done)
	return nil
}

func (l *WebsocketTransportListener) ensureStarted() error {
	if l.srv == nil {
		return errors.New("ws listener: listener is not started")
	}

	return nil
}

func (l *WebsocketTransportListener) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	conn, err := l.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	ws := &websocketTransport{
		conn: conn,
		c:    SessionCompressionNone,
	}

	if l.tls() {
		ws.e = SessionEncryptionTLS
	} else {
		ws.e = SessionEncryptionNone
	}

	l.transportChan <- ws
}
