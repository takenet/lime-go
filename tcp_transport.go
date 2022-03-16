package lime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
	"time"
)

const DefaultReadLimit int64 = 8192 * 1024

type tcpTransport struct {
	TCPConfig
	conn          net.Conn
	ctxConn       *ctxConn
	encoder       *json.Encoder
	decoder       *json.Decoder
	limitedReader io.LimitedReader
	encryption    SessionEncryption
	server        bool
	eof           bool
}

// DialTcp opens a TCP  transport connection with the specified URI.
func DialTcp(ctx context.Context, addr net.Addr, config *TCPConfig) (Transport, error) {
	if addr.Network() != "tcp" {
		return nil, errors.New("address network should be tcp")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, addr.Network(), addr.String())
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &defaultTCPConfig
	}

	t := tcpTransport{TCPConfig: *config}

	t.setConn(conn)
	t.encryption = SessionEncryptionNone
	return &t, nil
}

func (t *tcpTransport) SupportedCompression() []SessionCompression {
	return []SessionCompression{SessionCompressionNone}
}

func (t *tcpTransport) Compression() SessionCompression {
	return SessionCompressionNone
}

func (t *tcpTransport) SetCompression(_ context.Context, c SessionCompression) error {
	return fmt.Errorf("compression '%v' is not supported", c)
}

func (t *tcpTransport) SupportedEncryption() []SessionEncryption {
	return []SessionEncryption{SessionEncryptionNone, SessionEncryptionTLS}
}

func (t *tcpTransport) Encryption() SessionEncryption {
	return t.encryption
}

func (t *tcpTransport) SetEncryption(ctx context.Context, e SessionEncryption) error {
	if e == t.encryption {
		return nil
	}

	if e == SessionEncryptionNone {
		return errors.New("cannot downgrade from tls to none encryption")
	}

	if e == SessionEncryptionTLS && t.TLSConfig == nil {
		return errors.New("tls config must be defined")
	}

	var tlsConn *tls.Conn

	// https://github.com/FluuxIO/go-xmpp/blob/master/xmpp_transport.go#L80
	if t.server {
		tlsConn = tls.Server(t.conn, t.TLSConfig)
	} else {
		tlsConn = tls.Client(t.conn, t.TLSConfig)
	}

	var deadline time.Time
	var ok bool
	if deadline, ok = ctx.Deadline(); !ok {
		deadline = time.Now().Add(30 * time.Second)
	}

	if err := tlsConn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	if err := tlsConn.SetReadDeadline(deadline); err != nil {
		return err
	}

	// We convert existing connection to TLS
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	t.setConn(tlsConn)
	t.encryption = SessionEncryptionTLS
	return nil
}

func (t *tcpTransport) Send(ctx context.Context, e Envelope) error {
	if ctx == nil {
		panic("nil context")
	}

	if e == nil || reflect.ValueOf(e).IsNil() {
		panic("nil envelope")
	}

	if err := t.ensureOpen(); err != nil {
		return err
	}

	t.ctxConn.SetWriteContext(ctx)

	if err := t.encoder.Encode(e); err != nil {
		if errors.Is(err, io.EOF) {
			t.eof = true
		}
		return fmt.Errorf("tcp transport: send: %w", err)
	}

	return nil
}

func (t *tcpTransport) Receive(ctx context.Context) (Envelope, error) {
	if ctx == nil {
		panic("nil context")
	}

	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	t.ctxConn.SetReadContext(ctx)

	var raw rawEnvelope
	if err := t.decoder.Decode(&raw); err != nil {
		if errors.Is(err, io.EOF) {
			t.eof = true
		}
		return nil, fmt.Errorf("tcp transport: receive: %w", err)
	}

	t.limitedReader.N = t.ReadLimit
	return raw.ToEnvelope()
}

func (t *tcpTransport) Close() error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	err := t.ctxConn.Close()
	t.conn = nil
	return err
}

func (t *tcpTransport) Connected() bool {
	return t.conn != nil && !t.eof
}

func (t *tcpTransport) LocalAddr() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.LocalAddr()
}

func (t *tcpTransport) RemoteAddr() net.Addr {
	if t.conn == nil {
		return nil
	}
	return t.conn.RemoteAddr()
}

func (t *tcpTransport) setConn(conn net.Conn) {
	t.conn = conn
	t.ctxConn = NewCtxConn(conn, 5*time.Second, 5*time.Second)

	var writer io.Writer = t.ctxConn
	var reader io.Reader = t.ctxConn

	// Configure the trace writer, if defined
	tw := t.TraceWriter
	if tw != nil {
		writer = io.MultiWriter(writer, *tw.SendWriter())
		reader = io.TeeReader(reader, *tw.ReceiveWriter())
	}

	// Sets the encoder to be used for sending envelopes
	t.encoder = json.NewEncoder(writer)

	if t.ReadLimit == 0 {
		t.ReadLimit = DefaultReadLimit
	}

	// Using a LimitedReader to avoid the connection be
	// flooded with a large JSON which may cause
	// high memory usage.
	t.limitedReader = io.LimitedReader{
		R: reader,
		N: t.ReadLimit,
	}
	t.decoder = json.NewDecoder(&t.limitedReader)
}

func (t *tcpTransport) ensureOpen() error {
	if !t.Connected() {
		return errors.New("transport is not open")
	}

	return nil
}

type tcpTransportListener struct {
	TCPConfig
	listener net.Listener
	mu       sync.RWMutex
	connChan chan net.Conn
	done     chan struct{}
}

func NewTCPTransportListener(config *TCPConfig) TransportListener {
	if config == nil {
		config = &defaultTCPConfig
	}
	return &tcpTransportListener{TCPConfig: *config}
}

type TCPConfig struct {
	ReadLimit   int64       // ReadLimit defines the limit for buffered data in read operations.
	TraceWriter TraceWriter // TraceWriter sets the trace writer for tracing connection envelopes
	TLSConfig   *tls.Config
	ConnBuffer  int
}

var defaultTCPConfig = TCPConfig{}

func (l *tcpTransportListener) Listen(ctx context.Context, addr net.Addr) error {
	if addr.Network() != "tcp" {
		return errors.New("address network should be tcp")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener != nil {
		return errors.New("tcp listener is already started")
	}

	var lc net.ListenConfig
	listener, err := lc.Listen(ctx, "tcp", addr.String())
	if err != nil {
		return err
	}

	l.listener = listener
	l.done = make(chan struct{})
	l.connChan = make(chan net.Conn, l.ConnBuffer)

	go l.serve(listener)

	return nil
}

func (l *tcpTransportListener) serve(listener net.Listener) {
	defer close(l.connChan)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return
			default:
				log.Printf("tcp listener: serve: %v\n", err)
				return
			}
		} else {
			select {
			case <-l.done:
				return
			case l.connChan <- conn:
			}
		}
	}
}

func (l *tcpTransportListener) Accept(ctx context.Context) (Transport, error) {
	if err := l.ensureStarted(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("tcp listener: %w", ctx.Err())
	case <-l.done:
		return nil, errors.New("tcp listener closed")
	case conn, ok := <-l.connChan:
		if !ok {
			return nil, errors.New("tcp listener not serving")
		}
		transport := tcpTransport{
			TCPConfig:  l.TCPConfig,
			encryption: SessionEncryptionNone,
		}
		transport.server = true
		transport.ReadLimit = l.ReadLimit
		transport.setConn(conn)
		return &transport, nil
	}
}

func (l *tcpTransportListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.listener == nil {
		return errors.New("tcp listener is not started")
	}

	close(l.done)
	err := l.listener.Close()
	l.listener = nil

	return err
}

func (l *tcpTransportListener) ensureStarted() error {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.listener == nil {
		return errors.New("tcp listener is not started")
	}

	return nil
}

// ctcConn implement a net.conn with support for context cancellation.
type ctxConn struct {
	conn         net.Conn
	readTimeout  time.Duration
	writeTimeout time.Duration
	readCtx      context.Context
	readCancel   context.CancelFunc
	writeCtx     context.Context
	writeCancel  context.CancelFunc
}

func NewCtxConn(conn net.Conn, readTimeout time.Duration, writeTimeout time.Duration) *ctxConn {
	if conn == nil {
		panic("nil conn")
	}

	return &ctxConn{
		conn:         conn,
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
		readCtx:      context.Background(),
		writeCtx:     context.Background(),
	}
}

func (c *ctxConn) SetReadContext(ctx context.Context) {
	if ctx == nil {
		panic("nil read ctx")
	}
	if c.readCancel != nil {
		c.readCancel()
		c.readCancel = nil
	}
	c.readCtx = ctx
}

func (c *ctxConn) SetWriteContext(ctx context.Context) {
	if ctx == nil {
		panic("nil write ctx")
	}
	if c.writeCancel != nil {
		c.writeCancel()
		c.writeCancel = nil
	}
	c.writeCtx = ctx
}

func (c *ctxConn) Read(b []byte) (n int, err error) {
	for {
		if err = c.readCtx.Err(); err != nil {
			return 0, err
		}

		deadline := time.Now().Add(c.readTimeout)

		// Use the context deadline only if it is early then the default
		if ctxDeadline, ok := c.readCtx.Deadline(); ok && deadline.After(ctxDeadline) {
			deadline = ctxDeadline
		}

		if err = c.conn.SetReadDeadline(deadline); err != nil {
			return 0, err
		}

		n, err = c.conn.Read(b)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
			return 0, err
		}

		return n, nil
	}
}

func (c *ctxConn) Write(b []byte) (n int, err error) {
	for {
		if err = c.writeCtx.Err(); err != nil {
			return 0, err
		}

		deadline := time.Now().Add(c.writeTimeout)

		// Use the context deadline only if it is early then the default
		if ctxDeadline, ok := c.writeCtx.Deadline(); ok && deadline.After(ctxDeadline) {
			deadline = ctxDeadline
		}

		if err = c.conn.SetWriteDeadline(deadline); err != nil {
			return 0, err
		}

		n, err = c.conn.Write(b)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
			return 0, err
		}

		return n, nil
	}
}

func (c *ctxConn) Close() error {
	if c.readCancel != nil {
		c.readCancel()
	}

	if c.writeCancel != nil {
		c.writeCancel()
	}

	return c.conn.Close()
}

func (c *ctxConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *ctxConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *ctxConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	if err := c.SetWriteDeadline(t); err != nil {
		return err
	}

	return nil
}

func (c *ctxConn) SetReadDeadline(t time.Time) error {
	ctx, cancel := context.WithDeadline(context.Background(), t)
	c.SetReadContext(ctx)
	c.readCancel = cancel
	return nil
}

func (c *ctxConn) SetWriteDeadline(t time.Time) error {
	ctx, cancel := context.WithDeadline(context.Background(), t)
	c.SetWriteContext(ctx)
	c.writeCancel = cancel
	return nil
}
