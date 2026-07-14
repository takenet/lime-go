# LIME Protocol - C# to Go Equivalence Plan

**Document Version:** 1.0  
**Date:** December 30, 2025  
**Status:** Planning Phase  
**Current Branch:** feat/csharp-equivalence-pt0

---

## Executive Summary

This document outlines a strategic plan to bring the **lime-go** library to feature parity with **lime-csharp**. Based on comprehensive analysis, lime-go currently implements approximately **40-50%** of lime-csharp functionality. This plan provides a phased approach to close this gap.

### Current State

**✅ What lime-go has:**

- Core LIME protocol (Message, Notification, Command, Session)
- Session management (negotiation, authentication, establishment)
- TCP and WebSocket transports
- Basic authentication schemes (Guest, Plain, Key, Transport, External)
- Session options (compression: None/GZip, encryption: None/TLS)
- Basic documents (Text, JSON, DocumentContainer, DocumentCollection)
- Basic chat resources (Account, Contact, Presence, Receipt, Delegation - simplified)

**❌ What lime-go is missing:**

- 20+ messaging content types
- Redis transport
- Channel modules/middleware
- Rich resource types with full properties
- ~~ASP.NET Core equivalent (HTTP/web server integration)~~ ✅ Done (see 3.3)
- Advanced listeners and builders
- CLI tools and samples
- Load testing and benchmarking infrastructure

### Estimated Total Effort

**~800-1200 developer hours** across 4 major phases

---

## Phase 1: Foundation Enhancements (Priority: HIGH)

**Goal:** Establish the architectural foundation for advanced features  
**Estimated Effort:** 120-180 hours  
**Timeline:** 3-4 weeks

### 1.1 URI and Resource Path Management

**Files to Create:**

- `uri.go` - LimeUri type with parsing and validation
- `uri_templates.go` - Standardized resource path constants

**Implementation:**

```go
// uri.go
type LimeUri struct {
    Scheme   string
    Path     string
    Query    map[string]string
}

func ParseLimeUri(uri string) (*LimeUri, error)
func (u *LimeUri) String() string
func (u *LimeUri) Resource() string
```

**Constants to add:**

```go
// uri_templates.go
const (
    UriTemplateAccount  = "/account"
    UriTemplatePresence = "/presence"
    UriTemplateContacts = "/contacts"
    UriTemplateContact  = "/contacts/{identity}"
    UriTemplateGroups   = "/groups"
    UriTemplateGroup    = "/groups/{identity}"
    UriTemplateReceipt  = "/receipt"
)
```

**Benefits:**

- Standardizes resource addressing
- Enables RESTful command patterns
- Required for advanced features

**Effort:** 20 hours

---

### 1.2 Channel Module System (Middleware)

**Files to Create:**

- `channel_module.go` - Base interfaces and types
- `notify_receipt_module.go` - Auto-receipt notification module

**Implementation:**

```go
// channel_module.go
type EnvelopeProcessor interface {
    ProcessOutgoing(ctx context.Context, env Envelope) (Envelope, error)
    ProcessIncoming(ctx context.Context, env Envelope) (Envelope, error)
}

type MessageModule interface {
    OnSending(ctx context.Context, msg *Message) (*Message, error)
    OnReceiving(ctx context.Context, msg *Message) (*Message, error)
}

type ChannelModuleManager struct {
    messageModules      []MessageModule
    notificationModules []NotificationModule
    commandModules      []CommandModule
}
```

**Example Module:**

```go
// notify_receipt_module.go
type NotifyReceiptModule struct {
    channel Sender
    event   NotificationEvent
}

func (m *NotifyReceiptModule) OnReceiving(ctx context.Context, msg *Message) (*Message, error) {
    if msg.ID != "" && msg.From.IsValid() {
        notification := msg.Notification(m.event)
        _ = m.channel.SendNotification(ctx, notification)
    }
    return msg, nil
}
```

**Integration Points:**

- `channel.go` - Add module chain to send/receive paths
- `server_channel.go` - Builder methods to add modules
- `client_channel.go` - Builder methods to add modules

**Benefits:**

- Extensible envelope processing pipeline
- Automatic receipt notifications
- Cross-cutting concerns (logging, metrics, validation)
- Matches C# architecture

**Effort:** 40 hours

---

### 1.3 Enhanced Resource Types

**Files to Modify:**

- `chat/resources.go` - Expand Account, Contact, add missing properties

**Account Enhancements:**

```go
type Account struct {
    contact
    // Existing fields...
    FullName           string    `json:"fullName,omitempty"`
    AccessKey          string    `json:"accessKey,omitempty"`
    
    // New fields to add:
    IsTemporary        *bool     `json:"isTemporary,omitempty"`
    Password           string    `json:"password,omitempty"` // Base64
    OldPassword        string    `json:"oldPassword,omitempty"` // Base64
    InboxSize          *int      `json:"inboxSize,omitempty"`
    AllowAnonymous     *bool     `json:"allowAnonymousSender,omitempty"`
    AllowUnknown       *bool     `json:"allowUnknownSender,omitempty"`
    StoreContent       *bool     `json:"storeMessageContent,omitempty"`
    EncryptContent     *bool     `json:"encryptMessageContent,omitempty"`
    PublishToDirectory *bool     `json:"publishToDirectory,omitempty"`
}
```

**Contact Enhancements:**

```go
type Contact struct {
    contact
    // Existing fields...
    
    // New fields to add:
    SubscriptionStatus *SubscriptionStatus `json:"subscriptionStatus,omitempty"`
    LastUpdateDate     *time.Time          `json:"lastUpdateDate,omitempty"`
}

type SubscriptionStatus string

const (
    SubscriptionStatusSubscribed    = SubscriptionStatus("subscribed")
    SubscriptionStatusNotSubscribed = SubscriptionStatus("not-subscribed")
)
```

**ContactDocument (Base) Enhancements:**

```go
type contact struct {
    // Existing fields...
    
    // New fields to add:
    PhoneNumber    string         `json:"phoneNumber,omitempty"`
    CellPhone      string         `json:"cellPhoneNumber,omitempty"`
    Gender         *Gender        `json:"gender,omitempty"`
    TimeZone       string         `json:"timeZoneName,omitempty"`
    Culture        string         `json:"culture,omitempty"`
    Extras         map[string]any `json:"extras,omitempty"`
    Address        string         `json:"address,omitempty"`
    City           string         `json:"city,omitempty"`
    PostalCode     string         `json:"postalCode,omitempty"`
    Country        string         `json:"country,omitempty"`
    TaxDocument    string         `json:"taxDocument,omitempty"`
    CreationDate   *time.Time     `json:"creationDate,omitempty"`
}

type Gender string

const (
    GenderMale   = Gender("male")
    GenderFemale = Gender("female")
)
```

**Files to Create:**

- `chat/group.go` - Group and GroupMember types
- `chat/quota.go` - Quota resource type

**Group Implementation:**

```go
type Group struct {
    Identity  Identity   `json:"identity"`
    Name      string     `json:"name,omitempty"`
    Type      GroupType  `json:"type,omitempty"`
    Members   []GroupMember `json:"members,omitempty"`
}

type GroupType string

const (
    GroupTypePublic  = GroupType("public")
    GroupTypePrivate = GroupType("private")
)

type GroupMember struct {
    Identity Identity        `json:"identity"`
    Role     GroupMemberRole `json:"role,omitempty"`
}

type GroupMemberRole string

const (
    GroupMemberRoleOwner  = GroupMemberRole("owner")
    GroupMemberRoleAdmin  = GroupMemberRole("admin")
    GroupMemberRoleMember = GroupMemberRole("member")
)
```

**Benefits:**

- Full compatibility with C# resource types
- Enables rich contact/account management
- Supports group collaboration features

**Effort:** 30 hours

---

### 1.4 Error Handling Improvements

**Files to Create:**

- `lime_error.go` - Enhanced error types

**Implementation:**

```go
// lime_error.go
type LimeError struct {
    Reason    *Reason
    Operation string
    Err       error
}

func (e *LimeError) Error() string {
    if e.Reason != nil {
        return fmt.Sprintf("%s: [%d] %s", e.Operation, e.Reason.Code, e.Reason.Description)
    }
    return fmt.Sprintf("%s: %v", e.Operation, e.Err)
}

func (e *LimeError) Unwrap() error {
    return e.Err
}

func NewLimeError(operation string, reason *Reason, err error) *LimeError {
    return &LimeError{
        Operation: operation,
        Reason:    reason,
        Err:       err,
    }
}
```

**Benefits:**

- Better error context
- Easier debugging
- Consistent error handling

**Effort:** 10 hours

---

### 1.5 Builder Pattern Enhancements

**Files to Modify:**

- `client.go` - Enhanced ClientBuilder
- `server.go` - Enhanced ServerBuilder

**Additions:**

```go
// client.go
func (b *ClientBuilder) WithModules(modules ...MessageModule) *ClientBuilder
func (b *ClientBuilder) WithAutoReceipt(event NotificationEvent) *ClientBuilder
func (b *ClientBuilder) WithRetry(maxAttempts int, backoff time.Duration) *ClientBuilder

// server.go  
func (b *ServerBuilder) WithModules(modules ...MessageModule) *ServerBuilder
func (b *ServerBuilder) WithAutoReceipt(event NotificationEvent) *ServerBuilder
func (b *ServerBuilder) QuotaPerSecond(quota int) *ServerBuilder
```

**Benefits:**

- More ergonomic API
- Matches C# builder patterns
- Easier configuration

**Effort:** 20 hours

---

## Phase 2: Content Types (Priority: HIGH)

**Goal:** Implement messaging content types for rich interactions  
**Estimated Effort:** 200-280 hours  
**Timeline:** 5-7 weeks

### 2.1 Core Messaging Content Types

**Files to Create:**

- `contents/` directory (new package)
- `contents/chatstate.go`
- `contents/input.go`
- `contents/select.go`
- `contents/document_select.go`
- `contents/location.go`
- `contents/link.go` (base)
- `contents/media_link.go`
- `contents/web_link.go`

### 2.1.1 ChatState

**Implementation:**

```go
// contents/chatstate.go
package contents

import "github.com/takenet/lime-go"

type ChatState struct {
    State    ChatStateEvent `json:"state"`
    Interval *int           `json:"interval,omitempty"`
}

func (c *ChatState) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.chatstate",
        Suffix:  "json",
    }
}

type ChatStateEvent string

const (
    ChatStateStarting  = ChatStateEvent("starting")
    ChatStateComposing = ChatStateEvent("composing")
    ChatStatePaused    = ChatStateEvent("paused")
    ChatStateDeleting  = ChatStateEvent("deleting")
    ChatStateGone      = ChatStateEvent("gone")
)
```

**Tests:** `contents/chatstate_test.go`

**Effort:** 8 hours

---

### 2.1.2 Input (User Input Request)

**Implementation:**

```go
// contents/input.go
package contents

type Input struct {
    Label      *lime.DocumentContainer `json:"label,omitempty"`
    Validation *InputValidation        `json:"validation,omitempty"`
}

func (i *Input) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.input",
        Suffix:  "json",
    }
}

type InputValidation struct {
    Rule  InputValidationRule `json:"rule,omitempty"`
    Regex string             `json:"regex,omitempty"`
    Error string             `json:"error,omitempty"`
}

type InputValidationRule string

const (
    ValidationText     = InputValidationRule("text")
    ValidationNumber   = InputValidationRule("number")
    ValidationDate     = InputValidationRule("date")
    ValidationEmail    = InputValidationRule("email")
    ValidationPhoneNumber = InputValidationRule("phoneNumber")
    ValidationRegex    = InputValidationRule("regex")
)
```

**Tests:** `contents/input_test.go`

**Effort:** 12 hours

---

### 2.1.3 Select (Menu/Quick Replies)

**Implementation:**

```go
// contents/select.go
package contents

type Select struct {
    Scope   SelectScope    `json:"scope,omitempty"`
    Text    string         `json:"text,omitempty"`
    Options []SelectOption `json:"options,omitempty"`
}

func (s *Select) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.select",
        Suffix:  "json",
    }
}

type SelectScope string

const (
    SelectScopeImmediate = SelectScope("immediate")
    SelectScopePersistent = SelectScope("persistent")
)

type SelectOption struct {
    Order *int            `json:"order,omitempty"`
    Text  string          `json:"text,omitempty"`
    Type  *lime.MediaType `json:"type,omitempty"`
    Value lime.Document   `json:"value,omitempty"`
}
```

**Tests:** `contents/select_test.go`

**Effort:** 15 hours

---

### 2.1.4 DocumentSelect

**Implementation:**

```go
// contents/document_select.go
package contents

type DocumentSelect struct {
    Scope   SelectScope             `json:"scope,omitempty"`
    Header  *lime.DocumentContainer `json:"header,omitempty"`
    Options []DocumentSelectOption  `json:"options,omitempty"`
}

func (d *DocumentSelect) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.document-select",
        Suffix:  "json",
    }
}

type DocumentSelectOption struct {
    Label *lime.DocumentContainer `json:"label,omitempty"`
    Value *lime.DocumentContainer `json:"value,omitempty"`
}
```

**Tests:** `contents/document_select_test.go`

**Effort:** 12 hours

---

### 2.1.5 Location

**Implementation:**

```go
// contents/location.go
package contents

type Location struct {
    Latitude  float64 `json:"latitude"`
    Longitude float64 `json:"longitude"`
    Altitude  *float64 `json:"altitude,omitempty"`
    Accuracy  *float64 `json:"accuracy,omitempty"`
    Text      string   `json:"text,omitempty"`
}

func (l *Location) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.location",
        Suffix:  "json",
    }
}
```

**Tests:** `contents/location_test.go`

**Effort:** 8 hours

---

### 2.1.6 Links (MediaLink, WebLink)

**Implementation:**

```go
// contents/link.go (base class)
package contents

type link struct {
    Uri         string          `json:"uri"`
    PreviewUri  string          `json:"previewUri,omitempty"`
    PreviewType *lime.MediaType `json:"previewType,omitempty"`
    Title       string          `json:"title,omitempty"`
    Text        string          `json:"text,omitempty"`
}

// contents/media_link.go
type MediaLink struct {
    link
    Type        lime.MediaType `json:"type"`
    Size        *int64         `json:"size,omitempty"`
    AspectRatio string         `json:"aspectRatio,omitempty"`
}

func (m *MediaLink) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.media-link",
        Suffix:  "json",
    }
}

// contents/web_link.go
type WebLink struct {
    link
    Target WebLinkTarget `json:"target,omitempty"`
}

func (w *WebLink) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.web-link",
        Suffix:  "json",
    }
}

type WebLinkTarget string

const (
    WebLinkTargetSelf  = WebLinkTarget("self")
    WebLinkTargetBlank = WebLinkTarget("blank")
)
```

**Tests:** `contents/link_test.go`, `contents/media_link_test.go`, `contents/web_link_test.go`

**Effort:** 20 hours

---

### 2.2 Advanced Content Types

**Files to Create:**

- `contents/redirect.go`
- `contents/reply.go`
- `contents/reaction.go`
- `contents/document_list.go`
- `contents/document_tuple.go`
- `contents/sensitive_container.go`
- `contents/copy_paste.go`

### 2.2.1 Redirect (Conversation Handover)

**Implementation:**

```go
// contents/redirect.go
type Redirect struct {
    Address Node                    `json:"address"`
    Context *lime.DocumentContainer `json:"context,omitempty"`
}

func (r *Redirect) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.redirect",
        Suffix:  "json",
    }
}
```

**Effort:** 8 hours

---

### 2.2.2 Reply (Message Threading)

**Implementation:**

```go
// contents/reply.go
type Reply struct {
    Replied *lime.DocumentContainer `json:"replied"`
    InReplyTo *RepliedMessage       `json:"inReplyTo,omitempty"`
}

type RepliedMessage struct {
    ID   string     `json:"id"`
    From lime.Node  `json:"from,omitempty"`
    To   lime.Node  `json:"to,omitempty"`
    Type lime.MediaType `json:"type,omitempty"`
    Value lime.Document `json:"value,omitempty"`
}

func (r *Reply) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.reply",
        Suffix:  "json",
    }
}
```

**Effort:** 12 hours

---

### 2.2.3 Reaction (Emoji Reactions)

**Implementation:**

```go
// contents/reaction.go
type Reaction struct {
    InReplyTo *RepliedMessage `json:"inReplyTo,omitempty"`
    Emoji     string          `json:"emoji"`
}

func (r *Reaction) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.reaction",
        Suffix:  "json",
    }
}
```

**Effort:** 8 hours

---

### 2.2.4 DocumentList & DocumentTuple

**Implementation:**

```go
// contents/document_list.go
type DocumentList struct {
    Items []*lime.DocumentContainer `json:"items"`
    Total *int                      `json:"total,omitempty"`
}

func (d *DocumentList) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.list",
        Suffix:  "json",
    }
}

// contents/document_tuple.go
type DocumentTuple struct {
    Item1 *lime.DocumentContainer `json:"item1,omitempty"`
    Item2 *lime.DocumentContainer `json:"item2,omitempty"`
    Item3 *lime.DocumentContainer `json:"item3,omitempty"`
    Item4 *lime.DocumentContainer `json:"item4,omitempty"`
    Item5 *lime.DocumentContainer `json:"item5,omitempty"`
}

func (d *DocumentTuple) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.tuple",
        Suffix:  "json",
    }
}
```

**Effort:** 15 hours

---

### 2.2.5 SensitiveContainer (Encrypted Content)

**Implementation:**

```go
// contents/sensitive_container.go
type SensitiveContainer struct {
    Value     *lime.DocumentContainer `json:"value"`
    Encrypted bool                    `json:"encrypted,omitempty"`
}

func (s *SensitiveContainer) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.sensitive",
        Suffix:  "json",
    }
}
```

**Effort:** 10 hours

---

### 2.3 Payment Content Types

**Files to Create:**

- `contents/invoice.go`
- `contents/invoice_status.go`
- `contents/payment_receipt.go`
- `contents/payment_method.go`

### 2.3.1 Invoice

**Implementation:**

```go
// contents/invoice.go
type Invoice struct {
    Title    string           `json:"title,omitempty"`
    Created  *time.Time       `json:"created,omitempty"`
    Methods  []PaymentMethod  `json:"methods,omitempty"`
    DueTo    *time.Time       `json:"dueTo,omitempty"`
    Currency string           `json:"currency,omitempty"`
    Taxes    *float64         `json:"taxes,omitempty"`
    Total    float64          `json:"total"`
    Items    []InvoiceItem    `json:"items,omitempty"`
    BillTo   *PaymentAddress  `json:"billTo,omitempty"`
}

type InvoiceItem struct {
    Description string   `json:"description"`
    Quantity    float64  `json:"quantity"`
    Unit        string   `json:"unit,omitempty"`
    Currency    string   `json:"currency,omitempty"`
    Total       float64  `json:"total"`
}

type PaymentAddress struct {
    Street     string `json:"street,omitempty"`
    Number     string `json:"number,omitempty"`
    Complement string `json:"complement,omitempty"`
    District   string `json:"district,omitempty"`
    City       string `json:"city,omitempty"`
    PostalCode string `json:"postalCode,omitempty"`
    State      string `json:"state,omitempty"`
    Country    string `json:"country,omitempty"`
}

func (i *Invoice) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.invoice",
        Suffix:  "json",
    }
}
```

**Effort:** 20 hours

---

### 2.3.2 InvoiceStatus

**Implementation:**

```go
// contents/invoice_status.go
type InvoiceStatus struct {
    Status InvoiceStatusValue `json:"status"`
    Date   *time.Time         `json:"date,omitempty"`
    Code   string             `json:"code,omitempty"`
}

type InvoiceStatusValue string

const (
    InvoiceStatusCreated   = InvoiceStatusValue("created")
    InvoiceStatusCancelled = InvoiceStatusValue("cancelled")
    InvoiceStatusCompleted = InvoiceStatusValue("completed")
    InvoiceStatusRefunded  = InvoiceStatusValue("refunded")
)

func (i *InvoiceStatus) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.invoice-status",
        Suffix:  "json",
    }
}
```

**Effort:** 10 hours

---

### 2.3.3 PaymentReceipt

**Implementation:**

```go
// contents/payment_receipt.go
type PaymentReceipt struct {
    Payer      *PaymentContact  `json:"payer,omitempty"`
    Paid       *PaymentMethod   `json:"paid,omitempty"`
    Currency   string           `json:"currency,omitempty"`
    Total      float64          `json:"total"`
    Items      []InvoiceItem    `json:"items,omitempty"`
}

type PaymentContact struct {
    Name        string          `json:"name,omitempty"`
    Document    string          `json:"document,omitempty"`
    Email       string          `json:"email,omitempty"`
    PhoneNumber string          `json:"phoneNumber,omitempty"`
    Address     *PaymentAddress `json:"address,omitempty"`
}

func (p *PaymentReceipt) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "vnd.lime.payment-receipt",
        Suffix:  "json",
    }
}
```

**Effort:** 15 hours

---

### 2.3.4 PaymentMethod

**Implementation:**

```go
// contents/payment_method.go
type PaymentMethod struct {
    Name        string                `json:"name"`
    Account     *PaymentAccount       `json:"account,omitempty"`
    Installment *PaymentInstallment   `json:"installment,omitempty"`
}

type PaymentAccount struct {
    Number      string `json:"number,omitempty"`
    DisplayNumber string `json:"displayNumber,omitempty"`
    Provider    string `json:"provider,omitempty"`
}

type PaymentInstallment struct {
    Number int     `json:"number"`
    Total  float64 `json:"total"`
}
```

**Effort:** 10 hours

---

### 2.4 Text Content Types

**Files to Create:**

- `contents/plain_text.go`
- `contents/ssml_text.go`

**Implementation:**

```go
// contents/plain_text.go
type PlainText struct {
    Text string `json:"text"`
}

func (p *PlainText) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "text",
        Subtype: "plain",
        Suffix:  "",
    }
}

// contents/ssml_text.go
type SsmlText struct {
    Text string `json:"ssml"`
}

func (s *SsmlText) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "ssml",
        Suffix:  "xml",
    }
}
```

**Effort:** 8 hours

---

### 2.5 Document Registration

**Files to Modify:**

- `contents/init.go` - Register all content types

**Implementation:**

```go
// contents/init.go
package contents

import "github.com/takenet/lime-go"

func RegisterAll() {
    lime.RegisterDocumentFactory(func() lime.Document { return &ChatState{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Input{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Select{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &DocumentSelect{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Location{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &MediaLink{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &WebLink{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Redirect{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Reply{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Reaction{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &DocumentList{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &DocumentTuple{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &SensitiveContainer{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &Invoice{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &InvoiceStatus{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &PaymentReceipt{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &PlainText{} })
    lime.RegisterDocumentFactory(func() lime.Document { return &SsmlText{} })
}
```

**Effort:** 5 hours

---

## Phase 3: Transport & Infrastructure (Priority: MEDIUM)

**Goal:** Add Redis transport and listener abstractions  
**Estimated Effort:** 180-240 hours  
**Timeline:** 4-6 weeks

### 3.1 Redis Transport

**Dependencies:**

- `github.com/redis/go-redis/v9` - Redis client

**Files to Create:**

- `redis_transport.go`
- `redis_transport_listener.go`
- `redis_transport_test.go`

**Implementation:**

```go
// redis_transport.go
package lime

import (
    "context"
    "github.com/redis/go-redis/v9"
)

type RedisTransport struct {
    baseTransport
    client          *redis.Client
    channelPrefix   string
    sendChannel     string
    receiveChannel  string
    pubsub          *redis.PubSub
}

type RedisTransportConfig struct {
    Addr          string
    Password      string
    DB            int
    ChannelPrefix string
}

func NewRedisTransport(config *RedisTransportConfig) *RedisTransport {
    client := redis.NewClient(&redis.Options{
        Addr:     config.Addr,
        Password: config.Password,
        DB:       config.DB,
    })
    
    return &RedisTransport{
        client:        client,
        channelPrefix: config.ChannelPrefix,
    }
}

func (t *RedisTransport) Open(ctx context.Context) error {
    // Subscribe to receive channel
    t.pubsub = t.client.Subscribe(ctx, t.receiveChannel)
    return t.pubsub.Ping(ctx)
}

func (t *RedisTransport) Send(ctx context.Context, env Envelope) error {
    data, err := MarshalEnvelope(env)
    if err != nil {
        return err
    }
    return t.client.Publish(ctx, t.sendChannel, data).Err()
}

func (t *RedisTransport) Receive(ctx context.Context) (Envelope, error) {
    msg, err := t.pubsub.ReceiveMessage(ctx)
    if err != nil {
        return nil, err
    }
    return UnmarshalEnvelope([]byte(msg.Payload))
}

func (t *RedisTransport) Close(ctx context.Context) error {
    if t.pubsub != nil {
        _ = t.pubsub.Close()
    }
    return t.client.Close()
}
```

**RedisTransportListener:**

```go
// redis_transport_listener.go
type RedisTransportListener struct {
    config          *RedisTransportConfig
    client          *redis.Client
    accepting       bool
    acceptChan      chan Transport
}

func NewRedisTransportListener(config *RedisTransportConfig) *RedisTransportListener {
    return &RedisTransportListener{
        config:     config,
        acceptChan: make(chan Transport, 10),
    }
}

func (l *RedisTransportListener) Start(ctx context.Context) error {
    l.client = redis.NewClient(&redis.Options{
        Addr:     l.config.Addr,
        Password: l.config.Password,
        DB:       l.config.DB,
    })
    
    l.accepting = true
    go l.acceptLoop(ctx)
    return nil
}

func (l *RedisTransportListener) acceptLoop(ctx context.Context) {
    // Monitor for new connection requests via Redis
    // Create new RedisTransport instances for each connection
}

func (l *RedisTransportListener) Accept(ctx context.Context) (Transport, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case transport := <-l.acceptChan:
        return transport, nil
    }
}
```

**Benefits:**

- Pub/sub messaging patterns
- Scalable, distributed architectures
- Cloud-native deployments
- Matches C# Redis transport

**Effort:** 80 hours

---

### 3.2 Listener Abstractions

**Files to Create:**

- `listener.go` - Core listener interfaces
- `channel_listener.go` - Background envelope processing
- `buffered_channel_listener.go` - Buffered consumption
- `event_channel_listener.go` - Event-based handling

### 3.2.1 Core Listener Interface

**Implementation:**

```go
// listener.go
type ChannelListener interface {
    Start(ctx context.Context, channel Channel) error
    Stop() error
}

type MessageHandler func(ctx context.Context, msg *Message, sender Sender) error
type NotificationHandler func(ctx context.Context, not *Notification, sender Sender) error
type CommandHandler func(ctx context.Context, cmd *Command, sender Sender) error
```

**Effort:** 15 hours

---

### 3.2.2 Background Channel Listener

**Implementation:**

```go
// channel_listener.go
type BackgroundChannelListener struct {
    channel             Channel
    messageHandler      MessageHandler
    notificationHandler NotificationHandler
    commandHandler      CommandHandler
    
    stopChan chan struct{}
    wg       sync.WaitGroup
}

func NewBackgroundChannelListener(
    msgHandler MessageHandler,
    notHandler NotificationHandler,
    cmdHandler CommandHandler,
) *BackgroundChannelListener {
    return &BackgroundChannelListener{
        messageHandler:      msgHandler,
        notificationHandler: notHandler,
        commandHandler:      cmdHandler,
        stopChan:           make(chan struct{}),
    }
}

func (l *BackgroundChannelListener) Start(ctx context.Context, channel Channel) error {
    l.channel = channel
    
    l.wg.Add(3)
    go l.receiveMessages(ctx)
    go l.receiveNotifications(ctx)
    go l.receiveCommands(ctx)
    
    return nil
}

func (l *BackgroundChannelListener) receiveMessages(ctx context.Context) {
    defer l.wg.Done()
    for {
        select {
        case <-l.stopChan:
            return
        default:
            msg, err := l.channel.ReceiveMessage(ctx)
            if err != nil {
                continue
            }
            if l.messageHandler != nil {
                _ = l.messageHandler(ctx, msg, l.channel)
            }
        }
    }
}

func (l *BackgroundChannelListener) Stop() error {
    close(l.stopChan)
    l.wg.Wait()
    return nil
}
```

**Benefits:**

- Automatic envelope consumption
- Non-blocking receive loops
- Simplified server implementation

**Effort:** 40 hours

---

### 3.2.3 Event-Based Listener

**Implementation:**

```go
// event_channel_listener.go
type EventChannelListener struct {
    *BackgroundChannelListener
    
    OnMessage      []MessageHandler
    OnNotification []NotificationHandler
    OnCommand      []CommandHandler
}

func NewEventChannelListener() *EventChannelListener {
    l := &EventChannelListener{
        OnMessage:      make([]MessageHandler, 0),
        OnNotification: make([]NotificationHandler, 0),
        OnCommand:      make([]CommandHandler, 0),
    }
    
    l.BackgroundChannelListener = NewBackgroundChannelListener(
        l.handleMessage,
        l.handleNotification,
        l.handleCommand,
    )
    
    return l
}

func (l *EventChannelListener) handleMessage(ctx context.Context, msg *Message, sender Sender) error {
    for _, handler := range l.OnMessage {
        if err := handler(ctx, msg, sender); err != nil {
            return err
        }
    }
    return nil
}
```

**Benefits:**

- Event-driven architecture
- Multiple handlers per envelope type
- Matches C# EventChannelListener

**Effort:** 30 hours

---

### 3.3 HTTP/Web Server Integration

**Status:** ✅ **Done.** Implemented as a self-contained `http/` package that
implements the real, stateless LIME-over-HTTP protocol used by Blip's HTTP
gateway ("Iris", publicly exposed at `https://http.msging.net`), covering both
the outbound (client) and inbound (webhook/server) integration roles.

Unlike the original sketch below (which assumed wiring an HTTP handler
directly to a WebSocket-upgrading, session-oriented `lime.Server`), Iris'
actual HTTP integration is per-request and stateless: each call is
individually authenticated via `Authorization: Basic base64(identity:key)`
(or `Key` scheme) and carries a single envelope as its JSON body — there is no
session negotiation over HTTP. The implementation below reflects that real
protocol instead of the ASP.NET Core-coupled design originally sketched here.

**Files created:**

- `http/doc.go` - package documentation and usage overview
- `http/codec.go` - envelope type detection (mirrors `rawEnvelope.envelopeType()`)
  and JSON encode/decode helpers for `lime.Message`/`lime.Notification`/
  `lime.RequestCommand`/`lime.ResponseCommand`
- `http/auth.go` - Basic/Key `Authorization` header building (client) and
  parsing plus a pluggable `Authenticator` interface (server)
- `http/client.go` - `Client` type sending envelopes to Iris's `/messages`,
  `/notifications` and `/commands` endpoints, with synchronous
  `ResponseCommand` handling for commands
- `http/handler.go` - `Handler` type implementing `http.Handler`, receiving
  Iris-forwarded webhook envelopes, authenticating them and dispatching to
  pluggable `MessageHandler`/`NotificationHandler`/`CommandHandler` functions
- `http/client_test.go`, `http/handler_test.go`, `http/codec_test.go`,
  `http/auth_test.go` - table-driven tests using `net/http/httptest`
  (80.8% statement coverage)
- `examples/http-iris/client/main.go` - sends a message to Iris
- `examples/http-iris/server/main.go` - webhook server receiving Iris callbacks

**Usage:**

```go
// Outbound: sending to Iris
client := http.NewClient(identity, accessKey)
err := client.SendMessage(ctx, msg)

// Inbound: receiving webhooks forwarded by Iris
handler := &http.Handler{
    Authenticator: myAuthenticator,
    MessageHandler: func(ctx context.Context, msg *lime.Message) (*lime.Notification, error) {
        return msg.Notification(lime.NotificationEventConsumed), nil
    },
}
stdhttp.ListenAndServe(":8080", handler)
```

**Benefits:**

- Integration with Go web frameworks (mountable as a plain `http.Handler`)
- Interoperates directly with Blip/Iris's real HTTP gateway semantics
- No changes required to the existing session-oriented `Transport`/`Server`/
  `Client` core, keeping the change additive and low-risk

<details>
<summary>Original sketch (superseded by the implementation above)</summary>

**Files to Create:**

- `http/` directory (new package)
- `http/handler.go` - HTTP handler for LIME
- `http/middleware.go` - LIME middleware
- `http/websocket_upgrader.go` - WebSocket upgrade

**Implementation:**

```go
// http/handler.go
package http

import (
    "net/http"
    "github.com/takenet/lime-go"
)

type LimeHandler struct {
    server *lime.Server
}

func NewLimeHandler(server *lime.Server) *LimeHandler {
    return &LimeHandler{server: server}
}

func (h *LimeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Upgrade to WebSocket
    // Create transport
    // Pass to server
}

// Usage:
// server := lime.NewServerBuilder()...Build()
// handler := http.NewLimeHandler(server)
// http.ListenAndServe(":8080", handler)
```

**Benefits:**

- Integration with Go web frameworks
- REST + WebSocket on same server
- Equivalent to ASP.NET Core integration

**Effort:** 50 hours

</details>

---

## Phase 4: Tooling & Samples (Priority: LOW)

**Goal:** Developer experience improvements  
**Estimated Effort:** 150-200 hours  
**Timeline:** 4-5 weeks

### 4.1 CLI Tool

**Files to Create:**

- `cmd/lime-cli/` directory
- `cmd/lime-cli/main.go`
- `cmd/lime-cli/commands/` - Command implementations

**Features:**

- Connect to LIME servers
- Send messages/commands interactively
- Session management
- Template support
- Configuration files

**Implementation outline:**

```go
// cmd/lime-cli/main.go
package main

import (
    "github.com/spf13/cobra"
)

func main() {
    rootCmd := &cobra.Command{
        Use:   "lime-cli",
        Short: "LIME Protocol CLI client",
    }
    
    rootCmd.AddCommand(connectCmd())
    rootCmd.AddCommand(sendCmd())
    rootCmd.AddCommand(receiveCmd())
    
    rootCmd.Execute()
}
```

**Effort:** 60 hours

---

### 4.2 Enhanced Examples

**Directories to Create:**

- `examples/http-server/` - HTTP + WebSocket server
- `examples/redis-chat/` - Redis-based chat
- `examples/chatbot/` - Simple bot with content types
- `examples/payment-flow/` - Invoice/payment demo

**Effort:** 40 hours

---

### 4.3 Benchmarking Suite

**Files to Create:**

- `benchmark/` directory
- `benchmark/transport_bench_test.go`
- `benchmark/serialization_bench_test.go`
- `benchmark/throughput_bench_test.go`

**Implementation:**

```go
// benchmark/transport_bench_test.go
func BenchmarkTCPTransportSend(b *testing.B) {
    // Setup
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Send message
    }
}

func BenchmarkWebSocketTransportSend(b *testing.B) {
    // ...
}

func BenchmarkRedisTransportSend(b *testing.B) {
    // ...
}
```

**Effort:** 30 hours

---

### 4.4 Load Testing

**Files to Create:**

- `loadtest/` directory
- `loadtest/session_load_test.go`
- `loadtest/message_load_test.go`
- `loadtest/concurrent_clients_test.go`

**Effort:** 20 hours

---

## Phase 5: Documentation & Polish (Ongoing)

**Goal:** Complete documentation and API refinement  
**Estimated Effort:** 80-120 hours  
**Timeline:** Ongoing throughout phases

### 5.1 Documentation

**Files to Create/Update:**

- `docs/` directory
- `docs/getting-started.md`
- `docs/content-types.md`
- `docs/transports.md`
- `docs/authentication.md`
- `docs/modules.md`
- `docs/migration-from-csharp.md`

**Effort:** 40 hours

---

### 5.2 API Documentation

- Add comprehensive godoc comments
- Example code in documentation
- Tutorial-style guides

**Effort:** 30 hours

---

### 5.3 Code Quality

- Increase test coverage to >80%
- Add integration tests
- Performance optimizations
- Memory profiling

**Effort:** 30 hours

---

## Implementation Strategy

### Development Approach

1. **Feature Branches:** Each phase should be developed in separate branches
   - `feat/csharp-equivalence-pt0` - Foundation (current)
   - `feat/csharp-equivalence-pt1` - Content Types
   - `feat/csharp-equivalence-pt2` - Transports
   - `feat/csharp-equivalence-pt3` - Tooling

2. **Incremental PRs:** Break phases into smaller, reviewable PRs
   - Each content type as separate PR
   - Transport implementations separately
   - Don't merge until tests pass

3. **Backward Compatibility:** Maintain compatibility with existing code
   - Deprecate rather than remove
   - Add compatibility layer if needed

4. **Testing Requirements:**
   - Unit tests for all new code (>80% coverage)
   - Integration tests for transport implementations
   - Example code that demonstrates features

---

## Success Metrics

### Phase Completion Criteria

**Phase 1 (Foundation):**

- [ ] URI parsing and templates implemented
- [ ] Channel module system working
- [ ] Enhanced resource types with all properties
- [ ] Improved builders with module support
- [ ] Tests passing (>80% coverage)

**Phase 2 (Content Types):**

- [ ] All 20+ content types implemented
- [ ] Serialization/deserialization working
- [ ] Document registration complete
- [ ] Example usage for each type
- [ ] Tests passing (>80% coverage)

**Phase 3 (Transports):**

- [ ] Redis transport functional
- [ ] Listener abstractions complete
- [x] HTTP/WebSocket integration working
- [ ] Performance benchmarks acceptable
- [ ] Tests passing (>80% coverage)

**Phase 4 (Tooling):**

- [ ] CLI tool functional
- [ ] Enhanced examples working
- [ ] Benchmarking suite complete
- [ ] Load testing framework operational

**Phase 5 (Documentation):**

- [ ] All APIs documented
- [ ] Getting started guide complete
- [ ] Migration guide from C# available
- [ ] Tutorial examples working

---

## Risk Assessment

### Technical Risks

| Risk | Impact | Probability | Mitigation |
| ---- | ------ | ----------- | ---------- |
| Performance degradation with modules | Medium | Low | Benchmark early, optimize pipeline |
| Redis transport reliability | High | Medium | Extensive testing, fallback mechanisms |
| Breaking API changes | High | Low | Maintain compatibility layer |
| Serialization edge cases | Medium | Medium | Comprehensive test suite |

### Resource Risks

| Risk | Impact | Probability | Mitigation |
| ---- | ------ | ----------- | ---------- |
| Timeline overrun | Medium | High | Prioritize phases, cut scope if needed |
| Developer availability | High | Medium | Good documentation, modular design |
| Community adoption | Medium | Low | Marketing, examples, migration guides |

---

## Dependencies

### External Libraries Required

```go
// go.mod additions
require (
    github.com/redis/go-redis/v9 v9.x.x  // Phase 3
    github.com/spf13/cobra v1.x.x        // Phase 4 (CLI)
    github.com/spf13/viper v1.x.x        // Phase 4 (Config)
)
```

### Internal Dependencies

- Phase 2 depends on Phase 1 (URIs, modules)
- Phase 3 depends on Phase 1 (listeners need modules)
- Phase 4 depends on Phases 1-3 (tooling needs features)

---

## Timeline Summary

| Phase | Duration | Effort | Completion Target |
| ----- | -------- | ------ | ----------------- |
| Phase 1: Foundation | 3-4 weeks | 120-180 hours | Week 4 |
| Phase 2: Content Types | 5-7 weeks | 200-280 hours | Week 11 |
| Phase 3: Transports | 4-6 weeks | 180-240 hours | Week 17 |
| Phase 4: Tooling | 4-5 weeks | 150-200 hours | Week 22 |
| Phase 5: Documentation | Ongoing | 80-120 hours | Week 25 |

**Total Timeline:** ~25 weeks (6 months)  
**Total Effort:** ~800-1200 hours

---

## Conclusion

This plan provides a roadmap to achieve feature parity between lime-go and lime-csharp. The phased approach allows for:

- **Incremental value delivery** - Each phase provides usable features
- **Risk mitigation** - Early testing of critical components
- **Flexibility** - Phases can be adjusted based on priorities
- **Quality assurance** - Testing and documentation integrated throughout

### Immediate Next Steps

1. ✅ Review and approve this plan
2. ⬜ Begin Phase 1: Foundation Enhancements
   - Start with URI implementation
   - Then channel module system
   - Finally enhanced resource types
3. ⬜ Set up CI/CD for automated testing
4. ⬜ Create project board for tracking progress

### Long-term Vision

Once equivalence is achieved, lime-go will be positioned as a first-class LIME protocol implementation, suitable for:

- Production messaging applications
- Chatbot platforms
- Real-time communication systems
- IoT messaging infrastructure
- Distributed systems with Redis

The investment in this equivalence effort will pay dividends through increased adoption, community contribution, and production-readiness.

---

**Document Owner:** Development Team  
**Last Updated:** December 30, 2025  
**Next Review:** End of Phase 1
