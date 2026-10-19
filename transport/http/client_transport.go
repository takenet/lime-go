// Package httptransport implements the session-oriented HTTP transport used by Iris.
package httptransport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/takenet/lime-go/codec"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/tracing"
	"github.com/takenet/lime-go/transport"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	HeaderSession                    = "Http-Session"
	HeaderSessionID                  = "Http-Session-Id"
	HeaderSessionIdentity            = "Http-Session-Identity"
	HeaderSessionMessageURL          = "Http-Session-Message-Url"
	HeaderSessionNotificationURL     = "Http-Session-Notification-Url"
	HeaderSessionCommandURL          = "Http-Session-Command-Url"
	HeaderSessionMessageHeaders      = "Http-Session-Message-Headers"
	HeaderSessionNotificationHeaders = "Http-Session-Notification-Headers"
	HeaderSessionCommandHeaders      = "Http-Session-Command-Headers"
	HeaderSessionExpiration          = "Http-Session-Expiration"
	HeaderSessionPresenceRoutingRule = "Http-Session-Presence-Routing-Rule"
	HeaderSessionPresencePromiscuous = "Http-Session-Presence-Promiscuous"
	HeaderSessionReceiptEvents       = "Http-Session-Receipt-Events"
	HeaderShouldStore                = "server-Should-Store"
	HeaderCorrelationID              = "Correlation-Id"
	HeaderLimeSessionID              = "Lime-Session-Id"
	HeaderLimeRemoteNode             = "Lime-Remote-Node"
	HeaderLimeLocalNode              = "Lime-Local-Node"
)

const defaultMaxBodyBytes = 4 << 20

// CallbackAuthenticator validates requests delivered by Iris to this transport.
type CallbackAuthenticator func(*http.Request) bool

// Config configures an Iris HTTP session transport. ReceiveBaseUri enables
// callback delivery of incoming envelopes.
// Headers authenticate outbound requests, CallbackHeaders are sent to callback
// URLs, and CallbackAuthenticator independently validates incoming callbacks.
type Config struct {
	BaseURL                string
	Identity               protocol.Identity
	HTTPClient             *http.Client
	Headers                http.Header
	ReceiveBaseUri         string
	CallbackHeaders        http.Header
	CallbackAuthenticator  CallbackAuthenticator
	CallbackMux            *CallbackMux
	SessionExpiration      time.Duration
	SendSessionIDHeader    bool
	PresenceRoutingRule    string
	PresencePromiscuous    bool
	ReceiptEvents          []protocol.NotificationEvent
	CallbackCommandTimeout time.Duration
	MaxBodyBytes           int64
}

// ClientTransport implements transport.Transport over Iris HTTP sessions.
// ServeHTTP receives two-way session callbacks and can be mounted on the
// application HTTP server at the configured callback paths.
type ClientTransport struct {
	config         Config
	baseURL        *url.URL
	receiveBaseUri string
	callbackPath   string
	httpClient     *http.Client
	incoming       chan protocol.EnvelopeValue
	done           chan struct{}
	closeOnce      sync.Once

	mu            sync.RWMutex
	closed        bool
	httpSessionID string
	limeSessionID string
	remoteNode    protocol.Node
	localNode     protocol.Node

	pendingMu sync.Mutex
	pending   map[string]chan *protocol.ResponseCommand
}

var _ transport.Transport = (*ClientTransport)(nil)

func NewClientTransport(config Config) (*ClientTransport, error) {
	baseURL, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		return nil, errors.New("iris http transport: BaseURL must be an absolute HTTP or HTTPS URL")
	}
	if config.Identity.Name == "" || config.Identity.Domain == "" {
		return nil, errors.New("iris http transport: identity name and domain are required")
	}
	if config.HTTPClient == nil {
		config.HTTPClient = http.DefaultClient
	}
	if config.CallbackCommandTimeout <= 0 {
		config.CallbackCommandTimeout = 30 * time.Second
	}
	if config.MaxBodyBytes <= 0 {
		config.MaxBodyBytes = defaultMaxBodyBytes
	}
	if config.PresenceRoutingRule == "" {
		config.PresenceRoutingRule = "instance"
		if config.ReceiveBaseUri != "" {
			config.PresenceRoutingRule = "identity"
		}
	}
	if config.ReceiptEvents == nil {
		config.ReceiptEvents = []protocol.NotificationEvent{
			protocol.NotificationEventAccepted,
			protocol.NotificationEventReceived,
			protocol.NotificationEventConsumed,
			protocol.NotificationEventFailed,
		}
	}
	if config.ReceiveBaseUri != "" {
		callbackURL, parseErr := url.Parse(config.ReceiveBaseUri)
		if parseErr != nil || callbackURL.Host == "" || (callbackURL.Scheme != "http" && callbackURL.Scheme != "https") {
			return nil, errors.New("iris http transport: ReceiveBaseUri must be an absolute HTTP or HTTPS URL")
		}
	}
	config.Headers = config.Headers.Clone()
	forwarded := make(http.Header)
	for key, values := range config.CallbackHeaders {
		for _, value := range values {
			forwarded.Add(key, value)
		}
	}
	forwarded.Set(HeaderShouldStore, "true")
	forwarded.Set(HeaderSessionIdentity, config.Identity.String())
	if config.ReceiveBaseUri != "" {
		if err := validateCallbackHeaders(forwarded); err != nil {
			return nil, err
		}
	}
	config.CallbackHeaders = forwarded
	config.ReceiptEvents = append([]protocol.NotificationEvent{}, config.ReceiptEvents...)

	client := &ClientTransport{
		config:     config,
		baseURL:    baseURL,
		httpClient: config.HTTPClient,
		incoming:   make(chan protocol.EnvelopeValue, 64),
		done:       make(chan struct{}),
		pending:    make(map[string]chan *protocol.ResponseCommand),
	}
	if config.ReceiveBaseUri != "" {
		callbackBase, _ := url.Parse(config.ReceiveBaseUri)
		client.receiveBaseUri = strings.TrimRight(config.ReceiveBaseUri, "/")
		if config.CallbackMux != nil {
			client.callbackPath = strings.TrimRight(callbackBase.Path, "/") + "/" + uuid.NewString()
			client.receiveBaseUri = callbackBase.Scheme + "://" + callbackBase.Host + client.callbackPath
			config.CallbackMux.register(client.callbackPath, client)
		}
	}
	return client, nil
}

func (t *ClientTransport) Send(ctx context.Context, envelope protocol.EnvelopeValue) error {
	if ctx == nil {
		return errors.New("iris http transport: nil context")
	}
	if envelope == nil {
		return errors.New("iris http transport: nil envelope")
	}
	if !t.Connected() {
		return errors.New("iris http transport: transport is closed")
	}

	if response, ok := envelope.(*protocol.ResponseCommand); ok && t.submitCallbackResponse(response) {
		return nil
	}

	endpoint, expectResponse, initialSession, err := envelopeRequest(envelope)
	if err != nil {
		return err
	}
	data, err := codec.MarshalEnvelope(envelope)
	if err != nil {
		return fmt.Errorf("iris http transport: marshal envelope: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpointURL(endpoint), strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("iris http transport: build request: %w", err)
	}
	request.Header = t.requestHeaders(initialSession)
	propagation.TraceContext{}.Inject(ctx, propagation.HeaderCarrier(request.Header))
	if initialSession {
		if err := t.prepareSession(ctx, request.Header); err != nil {
			return err
		}
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := t.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("iris http transport: send %s: %w", endpoint, err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := t.readBody(response.Body)
	if err != nil {
		return fmt.Errorf("iris http transport: read %s response: %w", endpoint, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("iris http transport: %s returned status %d: %s", endpoint, response.StatusCode, string(body))
	}
	t.storeSessionID(response.Header.Get(HeaderSessionID))
	if !expectResponse {
		return nil
	}
	if len(body) == 0 {
		return fmt.Errorf("iris http transport: %s returned an empty response envelope", endpoint)
	}
	result, err := codec.UnmarshalEnvelope(body)
	if err != nil {
		return fmt.Errorf("iris http transport: decode %s response: %w", endpoint, err)
	}
	inheritHTTPContext(context.Background(), response.Header, result)
	if session, ok := result.(*protocol.Session); ok {
		t.storeLimeSession(session)
	}
	return t.enqueue(ctx, result)
}

func (t *ClientTransport) Receive(ctx context.Context) (protocol.EnvelopeValue, error) {
	if ctx == nil {
		return nil, errors.New("iris http transport: nil context")
	}
	if !t.Connected() {
		return nil, errors.New("iris http transport: transport is closed")
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("iris http transport: receive: %w", err)
	}
	select {
	case envelope := <-t.incoming:
		return envelope, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("iris http transport: receive: %w", ctx.Err())
	case <-t.done:
		return nil, errors.New("iris http transport: transport is closed")
	}
}

func (t *ClientTransport) Close() error {
	t.closeOnce.Do(func() {
		t.mu.Lock()
		t.closed = true
		t.mu.Unlock()
		close(t.done)
		if t.config.CallbackMux != nil {
			t.config.CallbackMux.unregister(t.callbackPath, t)
		}
	})
	return nil
}

func (t *ClientTransport) SupportedCompression() []protocol.SessionCompression {
	return []protocol.SessionCompression{protocol.SessionCompressionNone}
}

func (t *ClientTransport) Compression() protocol.SessionCompression {
	return protocol.SessionCompressionNone
}

func (t *ClientTransport) SetCompression(_ context.Context, compression protocol.SessionCompression) error {
	if compression != protocol.SessionCompressionNone {
		return errors.New("iris http transport: compression cannot be changed")
	}
	return nil
}

func (t *ClientTransport) SupportedEncryption() []protocol.SessionEncryption {
	return []protocol.SessionEncryption{t.Encryption()}
}

func (t *ClientTransport) Encryption() protocol.SessionEncryption {
	if t.baseURL.Scheme == "https" {
		return protocol.SessionEncryptionTLS
	}
	return protocol.SessionEncryptionNone
}

func (t *ClientTransport) SetEncryption(_ context.Context, encryption protocol.SessionEncryption) error {
	if encryption != t.Encryption() {
		return errors.New("iris http transport: encryption is determined by the URL scheme")
	}
	return nil
}

func (t *ClientTransport) Connected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return !t.closed
}

func (t *ClientTransport) LocalAddr() net.Addr {
	return endpointAddr{network: "iris-http", address: "client"}
}

func (t *ClientTransport) RemoteAddr() net.Addr {
	return endpointAddr{network: t.baseURL.Scheme, address: t.baseURL.String()}
}

func (t *ClientTransport) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !t.Connected() {
		http.Error(writer, "transport is closed", http.StatusServiceUnavailable)
		return
	}
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if t.config.CallbackAuthenticator != nil && !t.config.CallbackAuthenticator(request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	if sessionID := request.Header.Get(HeaderSessionID); sessionID != "" && sessionID != t.sessionID() {
		http.Error(writer, "invalid HTTP session", http.StatusUnauthorized)
		return
	}
	t.setCallbackResponseHeaders(writer, request)
	endpoint := lastPathSegment(request.URL.Path)
	if endpoint != "messages" && endpoint != "notifications" && endpoint != "commands" {
		http.NotFound(writer, request)
		return
	}

	body, err := t.readBody(http.MaxBytesReader(writer, request.Body, t.config.MaxBodyBytes))
	if err != nil {
		http.Error(writer, "invalid request body", http.StatusBadRequest)
		return
	}
	envelope, err := codec.UnmarshalEnvelope(body)
	if err != nil {
		http.Error(writer, "invalid LIME envelope", http.StatusBadRequest)
		return
	}
	if !envelopeMatchesEndpoint(envelope, endpoint) {
		http.Error(writer, "envelope type does not match callback URL", http.StatusBadRequest)
		return
	}

	inheritHTTPContext(request.Context(), request.Header, envelope)
	if command, ok := envelope.(*protocol.RequestCommand); ok && command.ID != "" {
		t.handleCommandCallback(writer, request, command)
		return
	}
	if err := t.enqueue(request.Context(), envelope); err != nil {
		http.Error(writer, "transport is closed", http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusAccepted)
}

func (t *ClientTransport) handleCommandCallback(writer http.ResponseWriter, request *http.Request, command *protocol.RequestCommand) {
	responseChannel := make(chan *protocol.ResponseCommand, 1)
	t.pendingMu.Lock()
	if _, exists := t.pending[command.ID]; exists {
		t.pendingMu.Unlock()
		http.Error(writer, "duplicate command ID", http.StatusConflict)
		return
	}
	t.pending[command.ID] = responseChannel
	t.pendingMu.Unlock()
	defer t.removePending(command.ID, responseChannel)

	if err := t.enqueue(request.Context(), command); err != nil {
		http.Error(writer, "transport is closed", http.StatusServiceUnavailable)
		return
	}
	timeout := time.NewTimer(t.config.CallbackCommandTimeout)
	defer timeout.Stop()
	select {
	case response := <-responseChannel:
		data, err := codec.MarshalEnvelope(response)
		if err != nil {
			http.Error(writer, "could not encode command response", http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/vnd.lime.command+json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(data)
	case <-request.Context().Done():
		http.Error(writer, request.Context().Err().Error(), http.StatusRequestTimeout)
	case <-timeout.C:
		http.Error(writer, "command response timeout", http.StatusGatewayTimeout)
	case <-t.done:
		http.Error(writer, "transport is closed", http.StatusServiceUnavailable)
	}
}

func inheritHTTPContext(ctx context.Context, headers http.Header, envelope protocol.EnvelopeValue) {
	if trace.SpanContextFromContext(tracing.ExtractContext(context.Background(), envelope)).IsValid() {
		return
	}
	if !trace.SpanContextFromContext(ctx).IsValid() {
		ctx = propagation.TraceContext{}.Extract(ctx, propagation.HeaderCarrier(headers))
	}
	tracing.InjectContext(ctx, envelope)
}

func (t *ClientTransport) requestHeaders(initialSession bool) http.Header {
	headers := t.config.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set(HeaderSessionIdentity, t.config.Identity.String())
	headers.Set(HeaderShouldStore, "true")
	if t.config.SendSessionIDHeader {
		if id := t.sessionID(); id != "" {
			headers.Set(HeaderSessionID, id)
		}
	}
	if initialSession {
		headers.Set(HeaderSessionPresenceRoutingRule, t.config.PresenceRoutingRule)
		headers.Set(HeaderSessionPresencePromiscuous, strconv.FormatBool(t.config.PresencePromiscuous))
		headers.Set(HeaderSessionReceiptEvents, notificationEvents(t.config.ReceiptEvents))
		if t.config.SessionExpiration > 0 {
			headers.Set(HeaderSessionExpiration, strconv.FormatInt(int64(t.config.SessionExpiration.Seconds()), 10))
		} else {
			headers.Set(HeaderSessionExpiration, "-1")
		}
		if t.receiveBaseUri != "" {
			base := strings.TrimRight(t.receiveBaseUri, "/")
			headers.Set(HeaderSessionMessageURL, base+"/messages")
			headers.Set(HeaderSessionNotificationURL, base+"/notifications")
			headers.Set(HeaderSessionCommandURL, base+"/commands")
			callbackHeaders := serializeHeaders(t.config.CallbackHeaders)
			headers.Set(HeaderSessionMessageHeaders, callbackHeaders)
			headers.Set(HeaderSessionNotificationHeaders, callbackHeaders)
			headers.Set(HeaderSessionCommandHeaders, callbackHeaders)
		}
	}
	return headers
}

func (t *ClientTransport) prepareSession(ctx context.Context, headers http.Header) error {
	headers.Del(HeaderSessionID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, t.endpointURL("sessions"), nil)
	if err != nil {
		return fmt.Errorf("iris http transport: build session preflight: %w", err)
	}
	request.Header = headers.Clone()
	request.Header.Del(HeaderSession)
	response, err := t.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("iris http transport: session preflight: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("iris http transport: session preflight returned status %d", response.StatusCode)
	}
	body, err := t.readBody(response.Body)
	if err != nil {
		return fmt.Errorf("iris http transport: read session preflight: %w", err)
	}
	envelope, err := codec.UnmarshalEnvelope(body)
	if err != nil {
		return fmt.Errorf("iris http transport: decode session preflight: %w", err)
	}
	session, ok := envelope.(*protocol.Session)
	if !ok {
		return errors.New("iris http transport: session preflight did not return a session")
	}
	if session.State != protocol.SessionStateEstablished || !sessionHeadersEqual(headers, response.Header) {
		headers.Set(HeaderSession, "reset")
	}
	return nil
}

func sessionHeadersEqual(expected, actual http.Header) bool {
	for _, key := range []string{
		HeaderSessionIdentity, HeaderSessionPresenceRoutingRule,
		HeaderSessionPresencePromiscuous, HeaderSessionReceiptEvents, HeaderSessionExpiration,
		HeaderSessionMessageURL, HeaderSessionNotificationURL, HeaderSessionCommandURL,
		HeaderSessionMessageHeaders, HeaderSessionNotificationHeaders, HeaderSessionCommandHeaders,
	} {
		wanted := strings.Join(expected.Values(key), ",")
		got := strings.Join(actual.Values(key), ",")
		if strings.HasSuffix(key, "-Url") || strings.HasSuffix(key, "-Headers") {
			if wanted != got {
				return false
			}
		} else if !strings.EqualFold(wanted, got) {
			return false
		}
	}
	return true
}

func (t *ClientTransport) endpointURL(endpoint string) string {
	return strings.TrimRight(t.baseURL.String(), "/") + "/" + endpoint
}

func (t *ClientTransport) storeSessionID(id string) {
	if id == "" {
		return
	}
	t.mu.Lock()
	if !t.closed {
		t.httpSessionID = id
	}
	t.mu.Unlock()
}

func (t *ClientTransport) storeLimeSession(session *protocol.Session) {
	if session.State != protocol.SessionStateEstablished {
		return
	}
	t.mu.Lock()
	t.limeSessionID = session.ID
	t.remoteNode = session.From
	t.localNode = session.To
	t.mu.Unlock()
}

func (t *ClientTransport) setCallbackResponseHeaders(writer http.ResponseWriter, request *http.Request) {
	if correlationID := request.Header.Get(HeaderCorrelationID); correlationID != "" {
		writer.Header().Set(HeaderCorrelationID, correlationID)
	}
	if sessionID := t.sessionID(); sessionID != "" {
		writer.Header().Set(HeaderSessionID, sessionID)
		writer.Header().Set(HeaderSessionIdentity, t.config.Identity.String())
	}
	t.mu.RLock()
	limeSessionID := t.limeSessionID
	remoteNode := t.remoteNode
	localNode := t.localNode
	t.mu.RUnlock()
	if limeSessionID != "" {
		writer.Header().Set(HeaderLimeSessionID, limeSessionID)
	}
	if remoteNode != (protocol.Node{}) {
		writer.Header().Set(HeaderLimeRemoteNode, remoteNode.String())
	}
	if localNode != (protocol.Node{}) {
		writer.Header().Set(HeaderLimeLocalNode, localNode.String())
	}
}

func (t *ClientTransport) sessionID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.httpSessionID
}

func (t *ClientTransport) enqueue(ctx context.Context, envelope protocol.EnvelopeValue) error {
	select {
	case t.incoming <- envelope:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return errors.New("transport is closed")
	}
}

func (t *ClientTransport) submitCallbackResponse(response *protocol.ResponseCommand) bool {
	t.pendingMu.Lock()
	responseChannel, ok := t.pending[response.ID]
	if ok {
		delete(t.pending, response.ID)
	}
	t.pendingMu.Unlock()
	if !ok {
		return false
	}
	select {
	case responseChannel <- response:
	case <-t.done:
	}
	return true
}

func (t *ClientTransport) removePending(id string, expected chan *protocol.ResponseCommand) {
	t.pendingMu.Lock()
	if t.pending[id] == expected {
		delete(t.pending, id)
	}
	t.pendingMu.Unlock()
}

func (t *ClientTransport) readBody(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, t.config.MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > t.config.MaxBodyBytes {
		return nil, errors.New("response body exceeds configured limit")
	}
	return data, nil
}

func envelopeRequest(envelope protocol.EnvelopeValue) (endpoint string, expectResponse, initialSession bool, err error) {
	switch value := envelope.(type) {
	case *protocol.Session:
		return "sessions", true, value.State == protocol.SessionStateNew, nil
	case *protocol.Message:
		return "messages", false, false, nil
	case *protocol.Notification:
		return "notifications", false, false, nil
	case *protocol.RequestCommand:
		return "commands", value.ID != "", false, nil
	case *protocol.ResponseCommand:
		return "commands", false, false, nil
	default:
		return "", false, false, fmt.Errorf("iris http transport: unsupported envelope type %T", envelope)
	}
}

func envelopeMatchesEndpoint(envelope protocol.EnvelopeValue, endpoint string) bool {
	switch endpoint {
	case "messages":
		_, ok := envelope.(*protocol.Message)
		return ok
	case "notifications":
		_, ok := envelope.(*protocol.Notification)
		return ok
	case "commands":
		_, request := envelope.(*protocol.RequestCommand)
		_, response := envelope.(*protocol.ResponseCommand)
		return request || response
	default:
		return false
	}
}

func notificationEvents(events []protocol.NotificationEvent) string {
	values := make([]string, len(events))
	for i, event := range events {
		values[i] = string(event)
	}
	return strings.Join(values, ",")
}

func serializeHeaders(headers http.Header) string {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(headers))
	for _, key := range keys {
		for _, value := range headers.Values(key) {
			values = append(values, key+"="+value)
		}
	}
	return strings.Join(values, ";")
}

func validateCallbackHeaders(headers http.Header) error {
	for key, values := range headers {
		invalidName := key == "" || strings.ContainsFunc(key, func(character rune) bool {
			return character <= ' ' || character >= 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", character)
		})
		if invalidName || len(values) != 1 {
			return errors.New("iris http transport: callback headers require valid names and single values")
		}
		if strings.ContainsFunc(values[0], func(character rune) bool {
			return character == ';' || character == 127 || (character < ' ' && character != '\t')
		}) {
			return errors.New("iris http transport: callback header value cannot contain delimiters or control characters")
		}
	}
	return nil
}

func lastPathSegment(path string) string {
	path = strings.Trim(path, "/")
	if index := strings.LastIndexByte(path, '/'); index >= 0 {
		return path[index+1:]
	}
	return path
}

type endpointAddr struct {
	network string
	address string
}

func (a endpointAddr) Network() string { return a.network }
func (a endpointAddr) String() string  { return a.address }
