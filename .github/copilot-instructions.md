# GitHub Copilot Instructions for lime-go

## Keep Instructions Durable

- Keep this file focused on contributor policies and decision-making guidance.
- Do not duplicate implementation details: package trees, API signatures, type definitions, constants, algorithms, or usage examples. Inspect the current source, package documentation, tests, and examples instead.
- Read [go.mod](../go.mod) for the supported Go version and dependencies, and [Makefile](../Makefile) for available build and validation targets. Do not copy their configuration here.
- Do not add migration phases, temporary workarounds, task history, or dated project snapshots to these instructions.

## Protocol Compatibility

- Maintain compatibility with the official [core protocol](https://limeprotocol.org/index.html), [content types](https://limeprotocol.org/content-types.html), and [resources](https://limeprotocol.org/resources.html) specifications.
- Consult the relevant specification before changing wire formats, serialization, session behavior, or resource semantics. Do not reproduce specification schemas or flows here.
- Preserve public API and wire compatibility unless a breaking change is explicitly requested.
- Use the [C# implementation](https://github.com/takenet/lime-csharp) as a secondary interoperability reference, not as a replacement for the specification. Surface discrepancies rather than silently choosing incompatible behavior.

## Implementation Principles

- Prioritize protocol correctness, then clarity and reliability, then performance. Optimize only with evidence.
- Follow idiomatic Go and nearby repository conventions. Prefer explicit control flow, meaningful names, focused interfaces, and existing abstractions over cleverness or new generic helpers.
- Locate the package that owns the behavior before editing. Follow current package boundaries rather than recreating historical layouts or duplicate implementations.
- Handle errors explicitly and preserve their identity when adding useful context. Prefer explicit returns over naked returns.
- Propagate caller cancellation and deadlines through blocking operations. Give goroutines a clear lifetime and ensure resources are released on failure and shutdown.
- Validate untrusted inputs, use safe defaults, and handle malformed data without panicking.
- Prefer dependency injection over new global mutable state. Avoid network calls, file I/O, or expensive work in initialization functions.
- Use structured logging consistent with the surrounding code. Never expose passwords, credentials, tokens, or other sensitive data in logs or error messages.
- Document exported behavior and non-obvious constraints in the owning package rather than restating implementation details here.

## Validation

- Add or update tests for changed behavior, including regression cases, invalid inputs, and relevant cancellation or failure paths.
- Follow nearby test conventions and use table-driven tests when scenarios share the same setup and assertions.
- Maintain at least 80% coverage for new code; coverage does not replace meaningful behavioral assertions.
- Start with focused tests for the affected packages, then run the applicable repository checks defined in the Makefile. Include race detection for concurrency changes.
- When moving code between packages, validate affected consumers and examples as well as the moved package.
- Report which checks ran and disclose any checks that could not be completed.# GitHub Copilot Instructions for lime-go

**Project:** LIME Protocol Implementation in Go  
**Version:** 1.25+  
**Last Updated:** December 30, 2025

---

## Project Overview

- Keep this file focused on contributor policies and decision-making guidance.
- Do not duplicate implementation details: package trees, API signatures, type definitions, constants, algorithms, or usage examples. Inspect the current source, package documentation, tests, and examples instead.
- Read [go.mod](../go.mod) for the supported Go version and dependencies, and [Makefile](../Makefile) for available build and validation targets. Do not copy their configuration here.
- Do not add migration phases, temporary workarounds, task history, or dated project snapshots to these instructions.

## Protocol Compatibility

- Maintain compatibility with the official LIME specifications:
    - [Core protocol](https://limeprotocol.org/index.html)
    - [Content types](https://limeprotocol.org/content-types.html)
    - [Resources](https://limeprotocol.org/resources.html)
- Consult the relevant specification before changing wire formats, serialization, session behavior, or resource semantics. Do not reproduce specification schemas or flows here.
- Preserve public API and wire compatibility unless a breaking change is explicitly requested.
- Use the [C# implementation](https://github.com/takenet/lime-csharp) as a secondary interoperability reference, not as a replacement for the specification. Surface discrepancies rather than silently choosing incompatible behavior.

## Implementation Principles

- Prioritize protocol correctness, then clarity and reliability, then performance. Optimize only with evidence.
- Follow idiomatic Go and nearby repository conventions. Prefer explicit control flow, meaningful names, focused interfaces, and existing abstractions over cleverness or new generic helpers.
- Locate the package that owns the behavior before editing. Follow current package boundaries rather than recreating historical layouts or duplicate implementations.
- Handle errors explicitly and preserve their identity when adding useful context. Prefer explicit returns over naked returns.
- Propagate caller cancellation and deadlines through blocking operations. Give goroutines a clear lifetime and ensure resources are released on failure and shutdown.
- Validate untrusted inputs, use safe defaults, and handle malformed data without panicking.
- Prefer dependency injection over new global mutable state. Avoid network calls, file I/O, or expensive work in initialization functions.
- Use structured logging consistent with the surrounding code. Never expose passwords, credentials, tokens, or other sensitive data in logs or error messages.
- Document exported behavior and non-obvious constraints in the owning package rather than restating implementation details here.

## Validation

- Add or update tests for changed behavior, including regression cases, invalid inputs, and relevant cancellation or failure paths.
- Follow nearby test conventions and use table-driven tests when scenarios share the same setup and assertions.
- Maintain at least 80% coverage for new code; coverage does not replace meaningful behavioral assertions.
- Start with focused tests for the affected packages, then run the applicable repository checks defined in the Makefile. Include race detection for concurrency changes.
- When moving code between packages, validate affected consumers and examples as well as the moved package.
- Report which checks ran and disclose any checks that could not be completed.

The `protocol`, `client`, and `server` packages implement LIME, a JSON-based open protocol for building scalable, real-time messaging applications. The implementation must maintain strict compatibility with the official LIME protocol specification.

### Official Specification References

**PRIMARY SOURCES - MUST BE FOLLOWED:**

1. **Core Protocol:** https://limeprotocol.org/index.html
   - Envelope types: Message, Notification, Command, Session
   - Session states and flow
   - Authentication schemes
   - Transport requirements

2. **Content Types:** https://limeprotocol.org/content-types.html
   - Standard document types and media types
   - Serialization formats
   - Content type registration

3. **Resources:** https://limeprotocol.org/resources.html
   - Standard resource URIs
   - Resource document schemas
   - Command methods and semantics

**CRITICAL:** Any implementation MUST conform to these specifications. When in doubt, refer to the specification first.

---

## Core Principles

### 1. Clarity Over Complexity

```go
// ✅ GOOD: Clear, explicit, easy to understand
func (c *ClientChannel) SendMessage(ctx context.Context, msg *Message) error {
    if msg == nil {
        return errors.New("message cannot be nil")
    }
    if !c.Established() {
        return errors.New("channel not established")
    }
    return c.transport.Send(ctx, msg)
}

// ❌ BAD: Overly clever, hard to understand
func (c *ClientChannel) SendMessage(ctx context.Context, msg *Message) error {
    return map[bool]func() error{
        true: func() error { return c.transport.Send(ctx, msg) },
        false: func() error { return errors.New("channel not established") },
    }[c.Established() && msg != nil]()
}
```

**Guidelines:**
- Write code that reads like prose
- Prefer explicit over implicit
- Use meaningful variable names
- Avoid clever tricks
- Comment complex logic, not obvious code

---

### 2. Resiliency and Reliability Over Performance

```go
// ✅ GOOD: Defensive, handles errors gracefully
func (t *WebSocketTransport) Receive(ctx context.Context) (Envelope, error) {
    // Set read deadline for resilience
    if deadline, ok := ctx.Deadline(); ok {
        if err := t.conn.SetReadDeadline(deadline); err != nil {
            return nil, fmt.Errorf("set read deadline: %w", err)
        }
    }
    
    // Read with buffer management
    buf := make([]byte, 8192)
    n, err := t.conn.Read(buf)
    if err != nil {
        if errors.Is(err, io.EOF) {
            return nil, ErrTransportClosed
        }
        return nil, fmt.Errorf("read from connection: %w", err)
    }
    
    // Parse envelope with validation
    env, err := UnmarshalEnvelope(buf[:n])
    if err != nil {
        return nil, fmt.Errorf("unmarshal envelope: %w", err)
    }
    
    return env, nil
}

// ❌ BAD: Optimized for speed but fragile
func (t *WebSocketTransport) Receive(ctx context.Context) (Envelope, error) {
    buf := make([]byte, 8192)
    n, _ := t.conn.Read(buf)  // Ignores errors!
    env, _ := UnmarshalEnvelope(buf[:n])  // Ignores errors!
    return env, nil
}
```

**Guidelines:**
- Check ALL errors
- Use context for cancellation and timeouts
- Validate inputs thoroughly
- Handle edge cases explicitly
- Prefer safe defaults
- Log errors with context
- Graceful degradation over crashes

---

### 3. Idiomatic Go (1.25+)

#### Error Handling

```go
// ✅ GOOD: Idiomatic error handling with wrapping
func (c *Channel) processCommand(ctx context.Context, cmd *Command) error {
    if err := c.validateCommand(cmd); err != nil {
        return fmt.Errorf("validate command: %w", err)
    }
    
    if err := c.executeCommand(ctx, cmd); err != nil {
        return fmt.Errorf("execute command: %w", err)
    }
    
    return nil
}

// ❌ BAD: Loses error context
func (c *Channel) processCommand(ctx context.Context, cmd *Command) error {
    if err := c.validateCommand(cmd); err != nil {
        return err  // No context!
    }
    
    if err := c.executeCommand(ctx, cmd); err != nil {
        return errors.New("command failed")  // Lost original error!
    }
    
    return nil
}
```

#### Context Usage

```go
// ✅ GOOD: Context-aware operations
func (s *Server) ListenAndServe(ctx context.Context) error {
    // Use context for shutdown
    go func() {
        <-ctx.Done()
        s.Close()
    }()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            conn, err := s.listener.Accept()
            if err != nil {
                if errors.Is(err, net.ErrClosed) {
                    return nil
                }
                return fmt.Errorf("accept connection: %w", err)
            }
            go s.handleConnection(ctx, conn)
        }
    }
}

// ❌ BAD: Ignores context
func (s *Server) ListenAndServe(ctx context.Context) error {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            return err
        }
        go s.handleConnection(ctx, conn)  // No cancellation handling!
    }
}
```

#### Struct Initialization

```go
// ✅ GOOD: Explicit field names
msg := &Message{
    Envelope: Envelope{
        ID:   "123",
        From: sender,
        To:   receiver,
    },
    Content: document,
}
    client := client.NewClientBuilder().
        Name("user").
            Domain("example.com").
msg := &Message{
            UseWebsocket("wss://127.0.0.1:55321", nil, nil).
    document,
    nil,
}
```

    Envelope: protocol.Envelope{ID: "1", To: receiver}}
    content := protocol.TextDocument("Hello!")
    msg.SetContent(&content)
```go
// ✅ GOOD: Small, focused interfaces
    channel.NoneCompressionSelector,
    channel.TLSEncryptionSelector,
    protocol.Identity{Name: "user", Domain: "example.com"},
    channel.GuestAuthenticator,
}

// ❌ BAD: God interface
type Channel interface {
    SendMessage(ctx context.Context, msg *Message) error
    ReceiveMessage(ctx context.Context) (*Message, error)
    SendNotification(ctx context.Context, not *Notification) error
    ReceiveNotification(ctx context.Context) (*Notification, error)
    SendCommand(ctx context.Context, cmd *Command) error
    ReceiveCommand(ctx context.Context) (*Command, error)
    Open(ctx context.Context) error
    Close() error
    State() SessionState
    SetState(SessionState)
    LocalNode() Node
    RemoteNode() Node
    // ... 20 more methods
}
```

#### Concurrency

```go
// ✅ GOOD: Use errgroup for coordinated goroutines
func (s *Server) handleConnections(ctx context.Context) error {
    g, ctx := errgroup.WithContext(ctx)
    
    for _, conn := range s.connections {
        conn := conn // Capture loop variable
        g.Go(func() error {
            return s.handleConnection(ctx, conn)
        })
    }
    
    return g.Wait()
}

// ❌ BAD: Uncoordinated goroutines
func (s *Server) handleConnections(ctx context.Context) error {
    for _, conn := range s.connections {
        go s.handleConnection(ctx, conn)  // No error handling!
    }
    return nil
}
```

---

## LIME Protocol Compliance

### Envelope Types

**MUST match specification exactly:**

```go
// Message - Transports content between nodes
type Message struct {
    Envelope
    Content Document  // Any document type
}

// Notification - Events associated with messages
type Notification struct {
    Envelope
    Event  NotificationEvent  // accepted, dispatched, received, consumed, failed
    Reason *Reason           // Present if Event == failed
}

// Command - RESTful operations on resources
type Command struct {
    Envelope
    Method   CommandMethod  // get, set, delete, merge, subscribe, unsubscribe, observe
    Uri      *LimeUri      // Resource identifier
    Type     *MediaType    // Resource type
    Resource Document      // Resource content
    Status   CommandStatus  // success, failure
    Reason   *Reason       // Present if Status == failure
}

// Session - Channel negotiation and establishment
type Session struct {
    Envelope
    State                SessionState           // new, negotiating, authenticating, established, finishing, finished, failed
    EncryptionOptions    []SessionEncryption    // Server offers
    Encryption           SessionEncryption      // Selected option
    CompressionOptions   []SessionCompression   // Server offers
    Compression          SessionCompression     // Selected option
    SchemeOptions        []AuthenticationScheme // Server offers
    Scheme               AuthenticationScheme   // Selected scheme
    Authentication       Authentication         // Auth data
    Reason               *Reason               // Present if State == failed
}
```

### Session Flow

**MUST follow specification flow:**

```go
// Client initiates with 'new' state
// 1. Client -> Server: Session{State: "new"}
// 2. Server -> Client: Session{State: "negotiating", CompressionOptions: [...], EncryptionOptions: [...]}
// 3. Client -> Server: Session{State: "negotiating", Compression: "gzip", Encryption: "tls"}
// 4. Server -> Client: Session{State: "authenticating", SchemeOptions: ["plain", "guest", ...]}
// 5. Client -> Server: Session{State: "authenticating", Scheme: "plain", Authentication: {...}}
// 6. Server -> Client: Session{State: "established", From: "user@domain/instance"}

func (c *ClientChannel) EstablishSession(
    ctx context.Context,
    compSelector CompressionSelector,
    encryptSelector EncryptionSelector,
    identity Identity,
    authenticator Authenticator,
    instance string,
) (*Session, error) {
    // Step 1: Send new session
    if err := c.sendNewSession(ctx); err != nil {
        return nil, fmt.Errorf("send new session: %w", err)
    }
    
    // Step 2: Receive negotiating session
    ses, err := c.receiveSession(ctx)
    if err != nil {
        return nil, fmt.Errorf("receive session: %w", err)
    }
    
    if ses.State != SessionStateNegotiating {
        return nil, fmt.Errorf("unexpected state: %s", ses.State)
    }
    
    // Step 3: Select and send options
    // ... (follow specification flow)
}
```

### Media Types

**MUST use specification-defined media types:**

```go
// Standard content types from https://limeprotocol.org/content-types.html
const (
    // Text
    MediaTypeTextPlain = "text/plain"
    
    // Application
    MediaTypeApplicationJson = "application/json"
    MediaTypeApplicationOctetStream = "application/octet-stream"
    
    // LIME-specific (vnd.lime.*)
    MediaTypeChatState      = "application/vnd.lime.chatstate+json"
    MediaTypeSelect         = "application/vnd.lime.select+json"
    MediaTypeInput          = "application/vnd.lime.input+json"
    MediaTypeLocation       = "application/vnd.lime.location+json"
    MediaTypeMediaLink      = "application/vnd.lime.media-link+json"
    MediaTypeWebLink        = "application/vnd.lime.web-link+json"
    MediaTypeCollection     = "application/vnd.lime.collection+json"
    MediaTypeContainer      = "application/vnd.lime.container+json"
    
    // Resources (from https://limeprotocol.org/resources.html)
    MediaTypeAccount    = "application/vnd.lime.account+json"
    MediaTypeContact    = "application/vnd.lime.contact+json"
    MediaTypePresence   = "application/vnd.lime.presence+json"
    MediaTypeReceipt    = "application/vnd.lime.receipt+json"
    MediaTypeCapability = "application/vnd.lime.capability+json"
    MediaTypeDelegation = "application/vnd.lime.delegation+json"
)
```

### Resource URIs

**MUST follow specification URI patterns:**

```go
// Standard resource paths from https://limeprotocol.org/resources.html
const (
    // Account resource
    UriAccount = "/account"  // GET/SET/DELETE user account
    
    // Presence resource
    UriPresence = "/presence"  // GET/SET/SUBSCRIBE presence status
    
    // Contacts (roster)
    UriContacts = "/contacts"  // GET collection, SET/DELETE individual
    
    // Groups
    UriGroups = "/groups"  // GET collection, SET/DELETE individual
    
    // Receipts configuration
    UriReceipt = "/receipt"  // SET notification preferences
    
    // Capabilities
    UriCapability = "/capability"  // GET node capabilities
    
    // Delegation
    UriDelegation = "/delegation"  // SET/GET/DELETE delegations
)

// URI with parameters (e.g., specific contact)
// Pattern: /contacts/{identity}
func ContactUri(identity Identity) string {
    return fmt.Sprintf("/contacts/%s", identity.String())
}
```

---

## Code Organization

### File Structure

```
lime-go/
├── *.go                    # Core protocol types (envelope, message, session, etc.)
├── *_test.go              # Unit tests (same package)
├── chat/                  # Chat-specific resources
│   ├── resources.go       # Account, Contact, Presence, etc.
│   └── resources_test.go
├── contents/              # Content types (Phase 2)
│   ├── chatstate.go
│   ├── input.go
│   ├── select.go
│   └── ...
├── examples/              # Usage examples
│   ├── client/
│   ├── server/
│   └── ws-chat/
├── http/                  # HTTP/WebSocket integration (Phase 3)
└── benchmark/             # Performance benchmarks (Phase 4)
```

### Package Naming

```go
// ✅ GOOD: Clear, standard package names
package lime          // Main protocol
package chat          // Chat resources
package contents      // Content types
package http          // HTTP integration

// ❌ BAD: Unclear or redundant names
package limeprotocol  // Redundant 'protocol'
package limechat      // Redundant 'lime'
package utils         // Too generic
package helpers       // Too generic
```

---

## Testing Requirements

### Unit Test Coverage

**MINIMUM 80% coverage required for all new code.**

```go
// ✅ GOOD: Comprehensive test with edge cases
func TestMessageNotification(t *testing.T) {
    tests := []struct {
        name     string
        message  *Message
        event    NotificationEvent
        wantErr  bool
    }{
        {
            name: "valid message and event",
            message: &Message{
                Envelope: Envelope{
                    ID:   "123",
                    From: Node{Identity: Identity{Name: "sender", Domain: "example.com"}},
                    To:   Node{Identity: Identity{Name: "receiver", Domain: "example.com"}},
                },
            },
            event:   NotificationEventReceived,
            wantErr: false,
        },
        {
            name:    "nil message",
            message: nil,
            event:   NotificationEventReceived,
            wantErr: true,
        },
        {
            name: "message without ID",
            message: &Message{
                Envelope: Envelope{
                    From: Node{Identity: Identity{Name: "sender", Domain: "example.com"}},
                },
            },
            event:   NotificationEventReceived,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            not := tt.message.Notification(tt.event)
            if tt.wantErr {
                assert.Nil(t, not)
            } else {
                assert.NotNil(t, not)
                assert.Equal(t, tt.message.ID, not.ID)
                assert.Equal(t, tt.event, not.Event)
            }
        })
    }
}

// ❌ BAD: Single happy path, no edge cases
func TestMessageNotification(t *testing.T) {
    msg := &Message{Envelope: Envelope{ID: "123"}}
    not := msg.Notification(NotificationEventReceived)
    assert.NotNil(t, not)
}
```

### Test Naming

```go
// Format: TestFunctionName_Condition_ExpectedBehavior

func TestClientChannelEstablishSession_WhenStateNew_ReturnsEstablishedSession(t *testing.T)
func TestClientChannelEstablishSession_WhenStateEstablished_ReturnsError(t *testing.T)
func TestTransportSend_WhenContextCanceled_ReturnsContextError(t *testing.T)
func TestSessionUnmarshal_WithInvalidJSON_ReturnsError(t *testing.T)
```

### Table-Driven Tests

```go
// ✅ GOOD: Use table-driven tests for multiple scenarios
func TestSessionStateValidate(t *testing.T) {
    tests := []struct {
        name    string
        state   SessionState
        wantErr bool
    }{
        {"new", SessionStateNew, false},
        {"negotiating", SessionStateNegotiating, false},
        {"authenticating", SessionStateAuthenticating, false},
        {"established", SessionStateEstablished, false},
        {"finishing", SessionStateFinishing, false},
        {"finished", SessionStateFinished, false},
        {"failed", SessionStateFailed, false},
        {"invalid", SessionState("invalid"), true},
        {"empty", SessionState(""), true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.state.Validate()
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

## Common Patterns

### Builder Pattern

```go
// ✅ GOOD: Fluent builder with validation
type ServerBuilder struct {
    config *ServerConfig
}

func NewServerBuilder() *ServerBuilder {
    return &ServerBuilder{
        config: &ServerConfig{
            CompOpts:   []SessionCompression{SessionCompressionNone},
            EncryptOpts: []SessionEncryption{SessionEncryptionNone},
            SchemeOpts: []AuthenticationScheme{},
        },
    }
}

func (b *ServerBuilder) ListenWebsocket(addr *net.TCPAddr, config *WebsocketConfig) *ServerBuilder {
    if addr == nil {
        panic("addr cannot be nil")
    }
    b.config.WebsocketAddr = addr
    b.config.WebsocketConfig = config
    return b
}

func (b *ServerBuilder) MessagesHandlerFunc(handler MessageHandlerFunc) *ServerBuilder {
    if handler == nil {
        panic("handler cannot be nil")
    }
    b.config.MsgHandler = handler
    return b
}

func (b *ServerBuilder) Build() *Server {
    // Validate configuration
    if b.config.WebsocketAddr == nil {
        panic("no transport configured")
    }
    
    return &Server{config: b.config}
}
```

### Options Pattern

```go
// ✅ GOOD: Functional options for complex configuration
type ClientOption func(*ClientConfig)

func WithRetry(maxAttempts int, backoff time.Duration) ClientOption {
    return func(c *ClientConfig) {
        c.MaxRetryAttempts = maxAttempts
        c.RetryBackoff = backoff
    }
}

func WithTimeout(timeout time.Duration) ClientOption {
    return func(c *ClientConfig) {
        c.Timeout = timeout
    }
}

func NewClient(opts ...ClientOption) *Client {
    config := &ClientConfig{
        MaxRetryAttempts: 3,
        RetryBackoff:     time.Second,
        Timeout:          30 * time.Second,
    }
    
    for _, opt := range opts {
        opt(config)
    }
    
    return &Client{config: config}
}

// Usage:
client := NewClient(
    WithRetry(5, 2*time.Second),
    WithTimeout(60*time.Second),
)
```

### Factory Pattern

```go
// ✅ GOOD: Factory for document type registration
type DocumentFactory func() Document

var documentFactories = make(map[MediaType]DocumentFactory)

func RegisterDocumentFactory(factory DocumentFactory) {
    doc := factory()
    mt := doc.MediaType()
    documentFactories[mt] = factory
}

func CreateDocument(mediaType MediaType) (Document, error) {
    factory, ok := documentFactories[mediaType]
    if !ok {
        return nil, fmt.Errorf("unknown media type: %s", mediaType)
    }
    return factory(), nil
}

// Usage:
func init() {
    RegisterDocumentFactory(func() Document { return &ChatState{} })
    RegisterDocumentFactory(func() Document { return &Location{} })
}
```

---

## Anti-Patterns to Avoid

### ❌ Global Mutable State

```go
// ❌ BAD: Global mutable state
var globalClient *Client

func SendMessage(msg *Message) error {
    return globalClient.SendMessage(context.Background(), msg)
}

// ✅ GOOD: Dependency injection
type MessageService struct {
    client *Client
}

func (s *MessageService) SendMessage(ctx context.Context, msg *Message) error {
    return s.client.SendMessage(ctx, msg)
}
```

### ❌ Ignoring Errors

```go
// ❌ BAD: Silently ignoring errors
func (c *Channel) Close() {
    c.transport.Close()  // Ignores error!
}

// ✅ GOOD: Handle or propagate errors
func (c *Channel) Close() error {
    if err := c.transport.Close(); err != nil {
        return fmt.Errorf("close transport: %w", err)
    }
    return nil
}
```

### ❌ Naked Returns

```go
// ❌ BAD: Naked returns are confusing
func (c *Channel) Send(ctx context.Context, env Envelope) (err error) {
    if env == nil {
        err = errors.New("envelope nil")
        return
    }
    err = c.transport.Send(ctx, env)
    return
}

// ✅ GOOD: Explicit returns
func (c *Channel) Send(ctx context.Context, env Envelope) error {
    if env == nil {
        return errors.New("envelope nil")
    }
    return c.transport.Send(ctx, env)
}
```

### ❌ Init() Side Effects

```go
// ❌ BAD: Heavy work in init()
func init() {
    // Don't do network calls, file I/O, or expensive operations in init()
    response, err := http.Get("https://example.com")
    if err != nil {
        panic(err)
    }
    globalResponse = response
}

// ✅ GOOD: Lightweight registration in init()
func init() {
    // Only register types, set defaults, etc.
    RegisterDocumentFactory(func() Document { return &ChatState{} })
}
```

---

## JSON Serialization

### Custom Marshaling

```go
// ✅ GOOD: Proper custom marshaling with error handling
func (m *Message) MarshalJSON() ([]byte, error) {
    raw, err := m.toRawEnvelope()
    if err != nil {
        return nil, fmt.Errorf("convert to raw envelope: %w", err)
    }
    
    data, err := json.Marshal(raw)
    if err != nil {
        return nil, fmt.Errorf("marshal to JSON: %w", err)
    }
    
    return data, nil
}

func (m *Message) UnmarshalJSON(data []byte) error {
    var raw rawEnvelope
    if err := json.Unmarshal(data, &raw); err != nil {
        return fmt.Errorf("unmarshal JSON: %w", err)
    }
    
    if err := m.populate(&raw); err != nil {
        return fmt.Errorf("populate from raw: %w", err)
    }
    
    return nil
}
```

### JSON Tags

```go
// ✅ GOOD: Proper JSON tags matching specification
type Message struct {
    Envelope
    Content Document `json:"content,omitempty"`
}

type ChatState struct {
    State    ChatStateEvent `json:"state"`             // Required field
    Interval *int           `json:"interval,omitempty"` // Optional field
}

// Use omitempty for optional fields
// Don't use omitempty for required fields
// Match field names to specification exactly
```

---

## Error Handling Strategy

### Error Types

```go
// ✅ GOOD: Sentinel errors for common cases
var (
    ErrTransportClosed   = errors.New("transport closed")
    ErrChannelNotEstablished = errors.New("channel not established")
    ErrInvalidState      = errors.New("invalid state")
    ErrTimeout           = errors.New("operation timeout")
)

// ✅ GOOD: Custom error types with context
type ValidationError struct {
    Field   string
    Value   interface{}
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s=%v: %s", e.Field, e.Value, e.Message)
}

// Usage with errors.Is() and errors.As()
if errors.Is(err, ErrTransportClosed) {
    // Handle closed transport
}

var validationErr *ValidationError
if errors.As(err, &validationErr) {
    // Handle validation error
}
```

### Error Context

```go
// ✅ GOOD: Add context at each layer
func (s *Server) handleSession(ctx context.Context, channel *ServerChannel) error {
    if err := channel.EstablishSession(ctx, ...); err != nil {
        return fmt.Errorf("establish session for %s: %w", channel.RemoteNode(), err)
    }
    
    if err := s.processMessages(ctx, channel); err != nil {
        return fmt.Errorf("process messages for %s: %w", channel.RemoteNode(), err)
    }
    
    return nil
}
```

---

## Logging Guidelines

### Structured Logging

```go
// ✅ GOOD: Use log/slog for structured logging (Go 1.21+)
import "log/slog"

func (c *ClientChannel) EstablishSession(ctx context.Context, ...) (*Session, error) {
    slog.InfoContext(ctx, "establishing session",
        "identity", identity.String(),
        "encryption", encryption,
        "compression", compression,
    )
    
    ses, err := c.negotiateSession(ctx, ...)
    if err != nil {
        slog.ErrorContext(ctx, "session negotiation failed",
            "identity", identity.String(),
            "error", err,
        )
        return nil, err
    }
    
    slog.InfoContext(ctx, "session established successfully",
        "session_id", ses.ID,
        "from", ses.From.String(),
    )
    
    return ses, nil
}
```

### Log Levels

- **Debug:** Detailed information for debugging
- **Info:** General informational messages (session established, message sent)
- **Warn:** Warning conditions (retry attempts, degraded mode)
- **Error:** Error conditions (failed operations, exceptions)

---

## Documentation Standards

### Package Documentation

```go
// ✅ GOOD: Package doc explaining purpose and usage
/*
Package protocol defines the LIME envelope and session models for the JSON-based
Lightweight Messaging Protocol.

The protocol specification is available at https://limeprotocol.org/

Basic usage:

	// Create a client
    client := client.NewClientBuilder().
        Name("user").
        Domain("example.com").
		PlainAuthentication("password").
        UseWebsocket("wss://127.0.0.1:55321", nil, nil).
		Build()

	// Establish session
	ctx := context.Background()
	session, err := client.EstablishSession(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Send message
    msg := &protocol.Message{Envelope: protocol.Envelope{ID: "1", To: receiver}}
    content := protocol.TextDocument("Hello!")
    msg.SetContent(&content)
	err = client.SendMessage(ctx, msg)

For more examples, see the examples/ directory.
*/
package protocol
```

### Function Documentation

```go
// ✅ GOOD: Clear documentation with examples
// EstablishSession performs the client session negotiation and authentication handshake
// as defined in the LIME protocol specification.
//
// The process involves:
//  1. Sending a 'new' session
//  2. Negotiating compression and encryption
//  3. Authenticating with the provided credentials
//
// Example:
//
//	session, err := channel.NewClientChannel(transport, 64).EstablishSession(
//		ctx,
//		channel.NoneCompressionSelector,
//		channel.TLSEncryptionSelector,
//		protocol.Identity{Name: "user", Domain: "example.com"},
//		channel.GuestAuthenticator,
//		"instance1",
//	)
func (c *ClientChannel) EstablishSession(
    ctx context.Context,
    compSelector CompressionSelector,
    encryptSelector EncryptionSelector,
    identity Identity,
    authenticator Authenticator,
    instance string,
) (*Session, error) {
    // Implementation...
}
```

---

## Performance Considerations

While clarity and reliability take priority, be mindful of:

### Memory Allocations

```go
// ✅ GOOD: Reuse buffers when possible
type Transport struct {
    readBuf []byte // Reused read buffer
}

func (t *Transport) Receive(ctx context.Context) (Envelope, error) {
    if cap(t.readBuf) < 8192 {
        t.readBuf = make([]byte, 8192)
    }
    t.readBuf = t.readBuf[:8192]
    
    n, err := t.conn.Read(t.readBuf)
    // ...
}

// ❌ BAD: Allocate new buffer every time
func (t *Transport) Receive(ctx context.Context) (Envelope, error) {
    buf := make([]byte, 8192)  // New allocation every call
    n, err := t.conn.Read(buf)
    // ...
}
```

### String Concatenation

```go
// ✅ GOOD: Use strings.Builder for multiple concatenations
func (i Identity) String() string {
    var b strings.Builder
    b.WriteString(i.Name)
    b.WriteByte('@')
    b.WriteString(i.Domain)
    return b.String()
}

// ❌ BAD: Multiple string concatenations
func (i Identity) String() string {
    return i.Name + "@" + i.Domain  // OK for simple cases
}
```

### Goroutine Management

```go
// ✅ GOOD: Bounded goroutine pools
type Server struct {
    workerPool chan struct{}  // Semaphore for limiting goroutines
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
    select {
    case s.workerPool <- struct{}{}:
        defer func() { <-s.workerPool }()
        // Handle connection
    case <-ctx.Done():
        return
    }
}

// ❌ BAD: Unbounded goroutines
func (s *Server) handleConnection(ctx context.Context, conn net.Conn) {
    go func() {  // Unlimited goroutines!
        // Handle connection
    }()
}
```

---

## Security Considerations

### Input Validation

```go
// ✅ GOOD: Validate all external inputs
func (c *Command) Validate() error {
    if c.Method == "" {
        return fmt.Errorf("method is required")
    }
    
    if !c.Method.IsValid() {
        return fmt.Errorf("invalid method: %s", c.Method)
    }
    
    if c.Uri == nil {
        return fmt.Errorf("uri is required")
    }
    
    // Additional validation...
    return nil
}
```

### Sensitive Data

```go
// ✅ GOOD: Don't log sensitive data
func (a *PlainAuthentication) String() string {
    return "PlainAuthentication{password: [REDACTED]}"
}

// ❌ BAD: Logging passwords
func (a *PlainAuthentication) String() string {
    return fmt.Sprintf("PlainAuthentication{password: %s}", a.Password)
}
```

---

## Quick Reference Checklist

When writing code, ensure:

- [ ] Follows LIME protocol specification exactly
- [ ] Uses idiomatic Go patterns
- [ ] Handles all errors explicitly
- [ ] Uses context for cancellation/timeout
- [ ] Validates all inputs
- [ ] Has comprehensive unit tests (>80% coverage)
- [ ] Includes documentation with examples
- [ ] No global mutable state
- [ ] Uses structured logging
- [ ] Graceful error handling
- [ ] Clear, readable code
- [ ] No premature optimization

---

## Additional Resources

- **LIME Protocol Specification:** https://limeprotocol.org/
- **Go Code Review Comments:** https://github.com/golang/go/wiki/CodeReviewComments
- **Effective Go:** https://go.dev/doc/effective_go
- **Go Proverbs:** https://go-proverbs.github.io/

---

## Questions or Clarifications

When uncertain about implementation details:

1. **Check the specification first:** https://limeprotocol.org/
2. **Look at existing code:** Find similar patterns in the codebase
3. **Consult C# implementation:** [`lime-csharp`](https://github.com/takenet/lime-csharp) repository for reference
4. **Refer to this document:** Follow the principles above
5. **Ask for clarification:** Better to ask than implement incorrectly

---

**Remember:** Write code that is correct first, clear second, and fast third. The LIME protocol correctness is non-negotiable. Code clarity makes maintenance easier. Performance can be optimized later if needed.
