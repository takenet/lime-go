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
	srv               *http.Server
	upgrader          *websocket.Upgrader
	transportBuffer   int
	transportChan     chan *websocketTransport
	errChan           chan error
	mu                sync.Mutex
}

func (t *WebsocketTransportListener) Listen(ctx context.Context, addr net.Addr) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.srv != nil {
		return errors.New("ws listener already started")
	}

	var lc net.ListenConfig
	l, err := lc.Listen(ctx, "tcp", addr.String())
	if err != nil {
		return err
	}

	t.transportChan = make(chan *websocketTransport, t.transportBuffer)
	t.errChan = make(chan error)
	t.upgrader = &websocket.Upgrader{
		Subprotocols:      []string{"lime"},
		EnableCompression: t.EnableCompression,
	}
	srv := &http.Server{
		Addr:      addr.String(),
		Handler:   t,
		TLSConfig: t.TLSConfig,
	}
	t.srv = srv

	go func() {
		if t.tls() {
			if err := srv.ServeTLS(l, t.CertFile, t.KeyFile); err != nil {
				t.errChan <- fmt.Errorf("ws listener: %w", err)
				_ = t.Close()
				return
			}
		} else {
			if err := srv.Serve(l); err != nil {
				t.errChan <- fmt.Errorf("ws listener: %w", err)
				_ = t.Close()
				return
			}
		}
	}()

	return nil
}

func (t *WebsocketTransportListener) tls() bool {
	return t.TLSConfig != nil || (t.CertFile != "" && t.KeyFile != "")
}

func (t *WebsocketTransportListener) Accept(ctx context.Context) (Transport, error) {
	if err := t.ensureStarted(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ws listener: %w", ctx.Err())
	case err := <-t.errChan:
		return nil, err
	case t, ok := <-t.transportChan:
		if !ok {
			return nil, errors.New("ws listener closed")
		}
		return t, nil
	}
}

func (t *WebsocketTransportListener) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.ensureStarted(); err != nil {
		return err
	}

	if err := t.srv.Close(); err != nil {
		return err
	}

	t.srv = nil
	close(t.transportChan)
	return nil
}

func (t *WebsocketTransportListener) ensureStarted() error {
	if t.srv == nil {
		return errors.New("ws listener: listener is not started")
	}

	return nil
}

func (t *WebsocketTransportListener) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	conn, err := t.upgrader.Upgrade(writer, request, nil)
	if err != nil {
		log.Println(err)
		return
	}

	ws := &websocketTransport{
		conn: conn,
		c:    SessionCompressionNone,
	}

	if t.tls() {
		ws.e = SessionEncryptionTLS
	} else {
		ws.e = SessionEncryptionNone
	}

	t.transportChan <- ws
}
