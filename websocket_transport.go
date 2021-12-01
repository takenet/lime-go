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

	"github.com/gorilla/websocket"
)

func DialWebsocket(ctx context.Context, urlStr string) (Transport, error) {
	d := websocket.Dialer{}

	requestHeader := http.Header{}
	requestHeader["Sec-WebSocket-Protocol"] = []string{"lime"}

	conn, _, err := d.DialContext(ctx, urlStr, requestHeader)
	if err != nil {
		return nil, err
	}

	return &websocketTransport{conn: conn, c: "", e: ""}, nil
}

type websocketTransport struct {
	conn *websocket.Conn
	c    SessionCompression
	e    SessionEncryption
}

func (t *websocketTransport) Close() error {
	if err := t.conn.Close(); err != nil {
		return err
	}

	t.conn = nil
	return nil
}

func (t *websocketTransport) Send(ctx context.Context, e Envelope) error {
	if ctx == nil {
		panic("nil context")
	}

	if e == nil || reflect.ValueOf(e).IsNil() {
		panic("nil envelope")
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

	var raw RawEnvelope

	if err := t.conn.ReadJSON(&raw); err != nil {
		return nil, err
	}

	return raw.ToEnvelope()
}

func (t *websocketTransport) GetSupportedCompression() []SessionCompression {
	return []SessionCompression{t.c}
}

func (t *websocketTransport) GetCompression() SessionCompression {
	return t.c
}

func (t *websocketTransport) SetCompression(ctx context.Context, c SessionCompression) error {
	panic("compression cannot be changed")
}

func (t *websocketTransport) GetSupportedEncryption() []SessionEncryption {
	return []SessionEncryption{t.e}
}

func (t *websocketTransport) GetEncryption() SessionEncryption {
	return t.e
}

func (t *websocketTransport) SetEncryption(ctx context.Context, e SessionEncryption) error {
	panic("encryption cannot be changed")
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

type WebsocketTransportListener struct {
	CertFile          string
	KeyFile           string
	TLSConfig         *tls.Config
	EnableCompression bool
	srv               *http.Server
	upgrader          *websocket.Upgrader
	transportBuffer   int
	transportChan     chan *websocketTransport
}

func (t *WebsocketTransportListener) Listen(_ context.Context, addr net.Addr) error {
	if addr.Network() != "ws" && addr.Network() != "wss" {
		return errors.New("address network should be ws or wss")
	}

	t.transportChan = make(chan *websocketTransport, t.transportBuffer)
	t.upgrader = &websocket.Upgrader{
		Subprotocols:      []string{"lime"},
		EnableCompression: t.EnableCompression,
	}

	t.srv = &http.Server{
		Addr:      addr.String(),
		Handler:   t,
		TLSConfig: t.TLSConfig,
	}

	switch addr.Network() {
	case "ws":
		if err := t.srv.ListenAndServe(); err != nil {
			return fmt.Errorf("ws listener: %w", err)
		}
	case "wss":
		if err := t.srv.ListenAndServeTLS(t.CertFile, t.KeyFile); err != nil {
			return fmt.Errorf("ws listener: %w", err)
		}
	default:
		panic("unknown addr network")
	}

	return nil
}

func (t *WebsocketTransportListener) Accept(ctx context.Context) (Transport, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("ws listener: %w", ctx.Err())
	case t, ok := <-t.transportChan:
		if !ok {
			return nil, errors.New("ws listener closed")
		}
		return t, nil
	}
}

func (t *WebsocketTransportListener) Close() error {
	if err := t.srv.Close(); err != nil {
		return err
	}

	close(t.transportChan)
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
	}
	t.transportChan <- ws
}
