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
)

const DefaultReadLimit int64 = 8192 * 1024

type tcpTransport struct {
	TCPConfig
	conn          net.Conn
	encoder       *json.Encoder
	decoder       *json.Decoder
	limitedReader io.LimitedReader
	encryption    SessionEncryption
	server        bool
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

	deadline, _ := ctx.Deadline() // Use the deadline zero value if ctx has no deadline defined
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

	// Sets the timeout for the next write operation
	deadline, _ := ctx.Deadline()
	if err := t.conn.SetWriteDeadline(deadline); err != nil {
		return err
	}
	// TODO: Handle context <-Done() signal
	// TODO: Encode writes a new line after each entry, how we can avoid this?
	return t.encoder.Encode(e)
}

func (t *tcpTransport) Receive(ctx context.Context) (Envelope, error) {
	if ctx == nil {
		panic("nil context")
	}

	if err := t.ensureOpen(); err != nil {
		return nil, err
	}

	// Sets the timeout for the next read operation
	deadline, _ := ctx.Deadline()
	if err := t.conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}

	var raw rawEnvelope

	// TODO: Handle context <-Done() signal
	if err := t.decoder.Decode(&raw); err != nil {
		return nil, err
	}

	// Reset the read limit
	t.limitedReader.N = t.ReadLimit

	return raw.ToEnvelope()
}

func (t *tcpTransport) Close() error {
	if err := t.ensureOpen(); err != nil {
		return err
	}

	err := t.conn.Close()
	t.conn = nil
	return err
}

func (t *tcpTransport) Connected() bool {
	return t.conn != nil
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

	var writer io.Writer = t.conn
	var reader io.Reader = t.conn

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
	if t.conn == nil {
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

	go l.serve()

	return nil
}

func (l *tcpTransportListener) serve() {
	defer close(l.connChan)

	for listener := l.listener; listener != nil; {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return
			default:
				log.Printf("tcp serve error: %v", err)
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
