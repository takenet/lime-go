
# LIME - A lightweight messaging library  

![Go](https://github.com/takenet/lime-go/workflows/Go/badge.svg?branch=main)

LIME allows you to build scalable, real-time messaging applications using a JSON-based
[open protocol](http://limeprotocol.org).
It's **fully asynchronous** and supports persistent WebSocket transports (WS/WSS).

You can send and receive any type of document into the wire as long it can be represented as JSON or text (plain or
encoded with base64) and it has a **MIME type** to allow the other party to handle it in the right way.

The connected nodes can send receipts to the other parties to notify events about messages (for instance, a message was
received or the content invalid or not supported).

Besides that, there's a **REST capable** command interface with verbs (*get, set, and delete*) and resource identifiers
(URIs) to allow rich messaging scenarios.
You can use that to provide services like on-band account registration or instance-messaging resources, like presence or
roster management.

Finally, it has built-in support for authentication, transport encryption, and compression.

## Getting started

Requires Go 1.27.2 or newer.

-----

### Server

Use `server.Server` and `server.NewServerBuilder()` to accept LIME connections.

A WebSocket transport listener should be configured.
You also should **register handlers** for processing the received envelopes.

The example below shows how to create a simple WebSocket server that echoes every received message to its originator:

```go

package main

import (
 "context"
 protocol "github.com/takenet/lime-go/protocol"
 limeserver "github.com/takenet/lime-go/server"
 "log"
 "net"
 "os"
 "os/signal"
 "syscall"
)

func main() {
    // Message handler that echoes all received messages to the originator
    msgHandler := func(ctx context.Context, msg *protocol.Message, s limeserver.Sender) error {
        echoMsg := &protocol.Message{}
        echoMsg.SetContent(msg.Content).SetTo(msg.From)
        return s.SendMessage(ctx, echoMsg)
    }
    
    // Build a server, listening for WebSocket connections on port 55321
    server := limeserver.NewServerBuilder().
        MessagesHandlerFunc(msgHandler).
        ListenWebsocket(&net.TCPAddr{Port: 55321}, &limeserver.WebsocketConfig{}).
        Build()
    
    // Listen for the OS termination signals
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigs
        if err := server.Close(); err != nil {
            log.Printf("close: %v\n", err)
        }
    }()
    
    // Start listening (blocking call)
     if err := server.ListenAndServe(); err != limeserver.ErrServerClosed {
        log.Printf("listen: %v\n", err)
    }
}
```

### Client

Use `client.Client` and `client.NewClientBuilder()` to connect to a LIME server.

```go
package main

import (
 "context"
 protocol "github.com/takenet/lime-go/protocol"
 limeclient "github.com/takenet/lime-go/client"
 "log"
 "time"
)

func main() {
    done := make(chan bool)
    
    // Defines a simple handler function for printing  
    // the received messages to the stdout
    msgHandler := func(ctx context.Context, msg *protocol.Message, s limeclient.Sender) error {
    if txt, ok := msg.Content.(protocol.TextDocument); ok {
        log.Printf("Text message received - ID: %v - Type: %v - Content: %v\n", msg.ID, msg.Type, txt)
    }
        close(done)
        return nil
    }
    
    // Initialize the client
    client := limeclient.NewClientBuilder().
        UseWebsocket("ws://127.0.0.1:55321", nil, nil).
        MessagesHandlerFunc(msgHandler).
        Build()
    
    // Prepare a simple text message to be sent
    msg := &protocol.Message{}
    msg.SetContent(protocol.TextDocument("Hello world!"))
    
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    // Send the message
    if err := client.SendMessage(ctx, msg); err != nil {
        log.Printf("send message: %v\n", err)
    }
    
    // Wait for the echo message
    <-done
    
    // Close the client
    err := client.Close()
    if err != nil {
        log.Printf("close: %v\n", err)
    }
}
```

### Fallback handlers and connection retries

`channel.EnvelopeMux` and `client.ClientBuilder` expose
`DefaultMessageHandlerFunc`, `DefaultNotificationHandlerFunc`, and
`DefaultRequestCommandHandlerFunc`, each taking the corresponding predicate and
handler function. Explicit handlers are always checked first, even if registered
after defaults. Within each group, registration order is first-match. A nil
predicate matches every envelope. Dispatch remains synchronous; a matched handler
returning an error stops the listener, without falling through to defaults.
Register handlers before starting the client/listener. `HandlerCounts` includes
both explicit and default registrations.

For example, a catch-all command fallback does not shadow native ping handling:

```go
builder := limeclient.NewClientBuilder().
    DefaultRequestCommandHandlerFunc(nil, handleApplicationCommand).
    AutoReplyPings().
    ConnectionRetryHandler(func(_ error, delay time.Duration) {
        log.Printf("LIME unavailable; retrying in %v", delay)
    })
```

`ClientConfig.ConnectionRetryHandler func(error, time.Duration)` is also available
without the builder. The SDK owns the retry loop and calls the hook synchronously
after each failed connection/session attempt that will be retried, with the error
and planned delay. The hook replaces default retry logging, must return promptly,
and must not call blocking client methods. Sanitize errors before logging them;
transport errors can contain credentials. With no hook, the SDK logs a generic
failure and delay without error text. Cancellation is not reported as a retry.
Backoff starts at zero, grows by `100ms * attempt^2` (zero-based), caps at 30
seconds, and resets after a successful connection. Waiting remains cancellable.

### HTTP integration (Iris)

Besides the persistent WebSocket transport, `lime-go` also ships a
`http` package for integrating with Blip's stateless HTTP gateway ("Iris",
publicly exposed at `https://http.msging.net`). Unlike WebSocket, LIME-
over-HTTP has no session negotiation: every request is individually
authenticated and carries a single envelope.

```go
import (
    protocol "github.com/takenet/lime-go/protocol"
    limehttp "github.com/takenet/lime-go/http"
)

// Sending a message to Iris
client := limehttp.NewClient(identity, accessKey)
err := client.SendMessage(ctx, msg)

// Receiving webhooks forwarded by Iris
handler := &limehttp.Handler{
    Authenticator: myAuthenticator,
    MessageHandler: func(ctx context.Context, msg *protocol.Message) (*protocol.Notification, error) {
        return msg.Notification(protocol.NotificationEventConsumed), nil
    },
}
http.ListenAndServe(":8080", handler)
```

For session-based Iris HTTP transport, use `transport/http` instead. It
performs session negotiation, sends the `Http-Session-*` headers, and receives
incoming envelopes through callbacks. Without `ReceiveBaseUri`, only outbound
requests and their synchronous responses are supported. Mount one `CallbackMux` on the externally reachable
callback URL and supply matching callback headers/authentication:

```go
import (
    "net/http"

    limeclient "github.com/takenet/lime-go/client"
    httptransport "github.com/takenet/lime-go/transport/http"
)

callbackMux := httptransport.NewCallbackMux()
http.Handle("/lime/", callbackMux)

sessionTransport := httptransport.Config{
    BaseURL:         "https://http.msging.net",
    ReceiveBaseUri:  "https://bot.example.com/lime",
    CallbackMux:     callbackMux,
    CallbackHeaders: http.Header{"Authorization": {"Bearer callback-token"}},
    CallbackAuthenticator: func(r *http.Request) bool {
        return r.Header.Get("Authorization") == "Bearer callback-token"
    },
}
client := limeclient.NewClientBuilder().
    Name("bot").
    Domain("example.com").
    UseHTTPTransport(sessionTransport).
    Build()
```

For `AuthenticationSchemeTransport`, configure the supplied `http.Client` with
the client certificate required by the Iris deployment. Callback URLs must be
reachable by Iris and served over HTTPS in production.

The client reuses its established channel; business sends do not poll or
renegotiate `/sessions`. Requests authenticate with `Http-Session-Identity`;
`SendSessionIDHeader` is opt-in. Iris HTTP session IDs and LIME channel IDs are
tracked separately.

An outbound envelope rejected with HTTP `401 Unauthorized` invalidates the
transport, unblocks its receiver and lets the client's existing reconnect loop
establish a fresh session and callbacks. The failed send still returns an error:
the library never replays it. Other HTTP failures (including `403`, `429` and
`5xx`), network errors and caller cancellation do not force session recreation or
retry the envelope. A network failure may occur after delivery; applications must
not assume it is safe to resend. Persistent authentication failures continue
through the existing bounded connection retry backoff until shutdown.

Hosts needing HTTP request-scoped execution may supply
`transport/http.Config.Dispatch func(context.Context, protocol.EnvelopeValue) error`.
It processes request commands, messages and notifications synchronously on the
request goroutine. Route these through `Client.Dispatch`, which snapshots the
established client channel, releases the client lock, attaches authoritative
session metadata and invokes the same EnvelopeMux handlers as ordinary delivery.
Publish the client safely before enabling callbacks; dispatch before session
establishment fails explicitly. Handler registration must finish first.

With `Dispatch` unset, HTTP delivery retains its existing queue behavior.
Response commands always stay on that queue so the independent transport
receiver can satisfy nested `ProcessCommand` calls while a business handler is
running. `Client.Dispatch` rejects response/session envelopes. Command callback
IDs are reserved before inline dispatch, duplicate IDs return HTTP 409, and
request cancellation, callback timeout and transport closure cancel the dispatch
context. Cancellation is cooperative: never retain host request objects or move
request-bound handling to another goroutine.

Outgoing request commands may receive an empty HTTP `202 Accepted` only when
`ReceiveBaseUri` is configured. `ProcessCommand` then waits for the matching
response envelope on `/commands`, under its caller's context/deadline. Empty
synchronous responses and empty session responses remain errors.
Asynchronous response callbacks must reach the instance that issued the command:
`ProcessCommand` correlation is in memory. A shared load-balanced callback URL
without instance affinity cannot reliably complete those waiters. Inline HTTP
responses do not need this affinity.

See the `http` package documentation and the `examples/http-iris/` directory
for complete client and server examples.

## OpenTelemetry

The `tracing` package uses the OpenTelemetry API with the `Lime.Protocol`
instrumentation scope, matching lime-csharp. Applications configure and shut down
their own tracer provider and exporters, for example with
`otel.SetTracerProvider(provider)`. The library does not install a provider or
export telemetry by itself.

Channels propagate W3C `traceparent` and `tracestate` through envelope metadata
when sending messages, notifications, commands, and sessions. Existing
`traceparent` values are preserved. Each send creates a producer span, continuing
the caller's trace (or the envelope's trace when there is no active caller), or
starting a new trace when neither exists. `EnvelopeMux` restores remote context
and creates consumer spans around handlers; `ProcessCommand` creates a client
span covering the request/response round trip. Ping commands skip handler and
round-trip spans, but still have send spans and propagate context.

Connection attempts and session negotiation have bounded spans that continue
an existing caller trace. Server negotiation prioritizes the initial session's
metadata. These contexts are not retained by the session's receive loop, so
unrelated envelopes without a parent start independent traces. Responses and
other work explicitly sent with a handler's context remain in that handler's trace.

HTTP callbacks prioritize valid envelope metadata, then the active HTTP request
span or W3C HTTP headers. Synchronous responses use W3C response headers
as a fallback when envelope metadata has no valid parent. HTTP sends and session
preflight propagate the active W3C context in request headers. Applications must
configure a tracer provider to create spans. Errors are recorded on channel spans.

Send and envelope-handler spans describe transmission and handling, not the
success of the response payload. Successfully sending or handling a failure
response does not mark those spans as errors; actual I/O and handler errors do.
Command-operation spans retain `lime.command.status`, add numeric
`lime.command.reason.code` and bounded `lime.command.outcome`, and leave known
rejections (not found, invalid argument, forbidden, unsupported resource/method)
unset. Processing and unknown failures remain errors. `SetEnvelopeTags` applies
operation status; `SetEnvelopeAttributes` records the same attributes without
changing status. These classifications do not change protocol responses.

For direct channel consumers or custom processing, start and end spans explicitly:

```go
import (
    "context"

    "github.com/takenet/lime-go/protocol"
    "github.com/takenet/lime-go/tracing"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

func handle(ctx context.Context, message *protocol.Message,
    process func(context.Context, *protocol.Message) error) error {
    ctx = tracing.ExtractContext(ctx, message)
    ctx, span := tracing.StartSpan(ctx, message, "Message.Handle",
        trace.WithSpanKind(trace.SpanKindConsumer))
    defer span.End()

    if err := process(ctx, message); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }
    tracing.SetEnvelopeTags(span, message)
    return nil
}
```

`StartSpan` prefers the active context by default. Use `ExtractContext` first to
prioritize the envelope's remote parent, as above. Invalid metadata falls back to
the caller's context without losing cancellation, deadlines, or context values.
New span context is injected into the envelope. Success status is finalized with
`SetEnvelopeTags` after processing, allowing processing errors to take precedence.
`InjectContext`, `InjectContextIfAbsent`, and `CopyContext` are also available for
custom propagation; replacing or copying context clears stale `tracestate`.

Envelope attributes use the C# keys and enum names: `lime.envelope.type`,
`lime.envelope.id`, `lime.envelope.from`, `lime.envelope.to`,
`lime.message.type`, `lime.notification.event`, `lime.command.uri`,
`lime.command.method`, `lime.command.type`, `lime.command.status`, and
`lime.session.state`. Request and response commands both use the envelope type
`Command`; enum labels use values such as `Get`, `Pending`, and `Success`.
Handlers include `lime.channel.local_node` and `lime.channel.remote_node` when
available; command round trips also include `lime.node` and
`lime.transport.local_endpoint`, `lime.transport.remote_endpoint`, and
`lime.transport.type`. Transport type values identify the Go implementation.

Envelope bodies, arbitrary metadata, and authentication are not captured.
Envelope IDs and node addresses are high-cardinality, potentially identifying
attributes; configure exporter filtering and retention for your environment.
Envelope metadata is mutable, so do not trace or send the same envelope
concurrently without synchronization. Export requires an application-configured
provider and a sampling policy that records the span.

## Protocol overview

The base protocol data package is called **envelope** and there are four types: **Message, notification, command and
session**.

All envelope types share some properties, like the `id` - the envelope's unique identifier - and the `from` and `to`
routing information.
They also have the optional `metadata` property, which can be used to send any extra information about the envelope,
much like a header in the HTTP protocol.

### Message

The message envelope is used to transport a **document** between sessions.
A document is just a type with a known MIME type.

For instance, a message with a **text document** can be represented like this in JSON:

```json
{
  "id": "1",
  "to": "john",
  "type": "text/plain",
  "content": "Hello from Lime!"
}
```

In Go, the message envelope is `protocol.Message`:

```go
msg := &protocol.Message{}
msg.SetContent(protocol.TextDocument("Hello from Lime!")).
    SetID("1").
    SetToString("john")
```

In this example, the document value is the `Hello from Lime!` text and its MIME type is `text/plain`.

This message also has an `id` property with `1` value.
The id value is useful to **correlate notifications** about the message.
When the id is set, the sender may receive notifications (receipts) with message events, which will have the same id.
For instance, you may want to know if a message was received or read by its destination.
In this case, you should provide an id value to the message.

The `to` property sets the destination address of the message, and it is used by the server to route the envelope
to the correct destination.
The address format is called **node** and in its full form is presented in the `name@domain/instance` format, similar to
the [XMPP's Jabber ID](https://xmpp.org/rfcs/rfc3920.html#rfc.section.3).
The node's *domain* and *instance* portions are optional, so the value `john` used in the example is a valid node
address.

In the previous example, the content is plain text.
But a message can be used to transport any type of document that can be represented as JSON.

For instance, to send a generic JSON document you can use the `application/json` type:

```json
{
  "id": "1",
  "to": "john",
  "type": "application/json",
  "content": {
    "text": "Hello from Lime!",
    "timestamp": "2022-03-23T00:00:00.000Z"
  }
}
```

Building the same message in Go would be like this:

```go
msg := &protocol.Message{}
msg.SetContent(&protocol.JsonDocument{
        "text": "Hello from Lime!",
        "timestamp": "2022-03-23T00:00:00.000Z"}).
    SetID("1").
    SetToString("john")
```

You can also can (and probably should) use custom MIME types for representing well-known types from your application
domain:

```json
{
  "id": "1",
  "to": "john",
  "type": "application/x-app-image+json",
  "content": {
    "caption": "Look at this kitten!",
    "url": "https://mycdn.com/cat.jpg"
  }
}
```

Using custom MIME types enables the mapping of documents with types from your code.
For this to be possible, these types need to implement `document.Document`.

```go
type Image struct {
    Caption string `json:"caption,omitempty"`
    URL     string `json:"url,omitempty"`
}

func (f *Image) MediaType() document.MediaType {
    return document.MediaType{
        Type:    "application",
        Subtype: "x-app-image",
        Suffix:  "json",
    }
}

// To register your custom type, use the RegisterDocumentFactory function.
func init() {
    document.RegisterDocumentFactory(func() document.Document {
        return &Image{}
    })
}
```
    msg.SetContent(protocol.TextDocument("Hello from Lime!")).
For instance, to send a message to the `john` node, use `SendMessage` on a
`server.Server` or `client.Client`:

```go
msg := &protocol.Message{}
msg.SetContent(protocol.TextDocument("Hello from Lime!")).
    SetID("1").
    SetToString("john")

err := client.SendMessage(context.Background(), msg)
```

And for receiving messages, you can use a message handler that can be registered during the instantiation of the client
or the server:

```go
client := limeclient.NewClientBuilder().
    MessagesHandlerFunc(
        func(ctx context.Context, msg *protocol.Message, s limeclient.Sender) error {
            if txt, ok := msg.Content.(protocol.TextDocument); ok {
                fmt.Printf("Text message received - ID: %v - Type: %v - Content: %v\n", msg.ID, msg.Type, txt) 
            }
            return nil
        }).
 Build()
```

### Notification

A notification provides information about a message to its sender.
They are sent only for messages that have the `id` value defined.

To illustrate, a node may want to notify the sender that a message was received.
It can be done like this:

```json
{
  "id": "1",
  "to": "mary",
  "event": "received"
}
```

The notification `to` value should have the value of the `from` property of the message (or the `pp` value, if present).

In Go, use `Notification(event)` on `*protocol.Message` to build a notification for the message:

```go
// Creates a corresponding notification to the message
if msg.ID != "" {
    not := msg.Notification(protocol.NotificationEventReceived)
    // Send the notification 
    err := s.SendNotification(ctx, not)
}
```

Notifications can be emitted by the **destination of the message or by intermediates** - like a server that routes the
message.

The protocol defines the following notification events:

- **accepted**: The message was received and accepted by an intermediate.
- **dispatched**: The message was dispatched to the destination by the intermediate.
- **received**: The message was received by its destination.
- **consumed**: The message was processed (read) by its destination.
- **failed**: A problem occurred during the processing of the message.

A single message can **have multiple notifications**, one or more for each hop on its path to the destination.

By convention, the **consumed** and **failed** notifications are considered final, so no other notification should be
received by the message sender after one of these.

In case of failed notifications, a **reason** value should be present.

For instance, a server (intermediate) should notify the sender if it is unable to determine the destination session of
a message:

```json
{
  "id": "1",
  "to": "mary",
  "event": "failed",
  "reason": {
    "code": 1,
    "description": "Destination not found"
  }
}
```

In Go, you can use the message's `FailedNotification(reason)` method for that:

```go
not := msg.FailedNotification(&protocol.Reason{Code: 1, Description: "Destination not found"})
```

### Command

The command envelope is used to **read and write resources of a remote node**.
It provides a REST capable interface, with URI and methods (verbs), similar to the HTTP protocol.
It also supports multiplexing, so the connection is not blocked to wait for a response when a request is sent.

There are two types of commands: a request command - which contains a `uri` value - or a response command - with a
`status` value.

For instance, you can use commands for managing your contact list or to set your current status (available, busy, away).
Another common use is **the in-band registration**, where users can create accounts for your service in the protocol
itself.

The advantage of using commands is that you can use the **same existing connection** that is used for messaging for
handling resources, instead of creating out-of-band connections for that.

In practice, you can avoid having an external HTTP service for handling resources related to your messaging service.

This is more efficient in terms of energy consumption but also is usually more performatic as well.
Using a session that is already established and authenticated avoids the additional overhead of a TLS handshake and
authentication that an external connection would require.

But there is a limitation: the command interface only supports JSON payloads, so you should not use it for
transporting binary or any kind of large content.

Like in an HTTP service, the URI and methods that you may use in commands depend on what the server implements.

For instance, a server could implement a contact management service.
In this example, to retrieve all contacts, you could send a command like this:

```json
{
  "id": "2",
  "method": "get",
  "uri": "/contacts"
}
```

And the server may respond to this request like this:

```json
{
  "id": "2",
  "from": "postmaster@localhost/server1",
  "method": "get",
  "status": "success",
  "type": "application/vnd.lime.collection+json",
  "resource": {
    "total": 2,
    "itemType": "application/vnd.lime.contact+json",
    "items": [
      {
        "identity": "john@localhost",
        "name": "John Doe"
      },
      {
        "identity": "mary@localhost",
        "name": "Mary Jane"
      }
    ]
  }
}
```

This is a response command with a **status** and a **resource** value.

Note that the value of the `id` property is the same as the request.
This is how we know that a response is to a specific request, so it is important to avoid using duplicate ids to avoid
collisions. A way of doing this is to use GUID (UUID) values as id for the requests.

The status is always present in a response command, but the resource may be present depending on the method of the
request and the status of the response. In successful `get` methods, the value of `resource` - and consequently `type` -
should be present. In `set` requests, the `resource` value will probably not be present. This is similar to the HTTP
methods and body, when `GET` requests will have a value in the response body if successful and not always in `POST`
requests.

In case of `failure` response status, the command should have the `reason` property defined:

```json
{
  "id": "2",
  "from": "postmaster@localhost/server1",
  "method": "get",
  "status": "failure",
  "reason": {
    "code": 10,
    "description": "No contact was found" 
  }
}
```

For creating a request command in Go, use `protocol.RequestCommand`:

```go
reqCmd := &protocol.RequestCommand{}
reqCmd.SetURIString("/contacts").
    SetMethod(protocol.CommandMethodGet).
    SetID(protocol.NewEnvelopeID())
```

Note that for the `id` value, we are using the value returned by `protocol.NewEnvelopeID()`, which returns
a UUID v4 string (something like `3cdd2654-911d-497e-834a-3b7865510155`).

For sending a command request, you should use the `ProcessCommand` method instead of the `SendCommand` method.
This is because the `ProcessCommand` method takes care of waiting for the corresponding response command.

```go
respCmd, err := client.ProcessCommand(context.Background(), reqCmd)
if err == nil {
    // TODO: Handle the response
}
```

In the server side, you can add handlers for specific commands using the `RequestCommandHandler*` methods from
the `server.Server` type.

```go
server := serverpkg.NewServerBuilder().
    RequestCommandHandlerFunc(
        // Set a predicate for filtering only the get contacts commands
        func(cmd *protocol.RequestCommand) bool {
            return cmd.Method == protocol.CommandMethodGet && cmd.URI.Path() == "/contacts"
        },
        // The handler implementation
        func(ctx context.Context, cmd *protocol.RequestCommand, s serverpkg.Sender) error {
            // Create a document collection of contacts
            contacts := &protocol.DocumentCollection{
                Total:    2,
                ItemType: resources.MediaTypeContact(),
                Items: []protocol.Document{
                    &resources.Contact{Name: "John Doe"},
                    &resources.Contact{Name: "Mary"},
                },
            }
            // Send the response to the sender
            respCmd := cmd.SuccessResponseWithResource(contacts)
            return s.SendResponseCommand(ctx, respCmd)
        }).
    // TODO: Setup other server options
    Build()
```

This is also valid for `client.Client`.
In Lime, the **client can receive and process commands requests** from other nodes, like the server.

### Session

> Note: The session establishment flow is automatically handled by the library.

The session envelope is used for the negotiation, authentication, and establishment of the communication channel between
the client and a server.
It helps the parties to select the transport options, like compression and encryption (TLS), authentication credentials,
and session metadata, like its `id` and local/remote node addresses.

The first envelope sent in every Lime session is the **new session** envelope, which the client sends to the server
after the transport connection is established:

```json
{
  "state": "new"
}
```

The server should reply to this with another session envelope, according to the session state that it wants to enforce.

For instance, the server may want to present the client with the transport options for negotiation.
WebSocket TLS is selected when the connection is created: `ws://` is unencrypted and `wss://` uses TLS. The LIME
session does not upgrade a WebSocket connection to TLS.

```json
{
  "id": "0676a702-a7d6-43e6-947f-bde3c3e25eb5",
  "from": "server@localhost/s1",
  "state": "negotiating",
  "compressionOptions": ["none"],
  "encryptionOptions": ["none", "tls"]
}
```

Note that this envelope haves an `id` defined, which is the **session id**.
The next session envelopes sent by the client should use this same id, until the end of the session.
During the session establishment, only session envelopes are allowed.

The server can skip the `negotiating` state and jump directly to the `authenticating` or even to the `established`
state. The session state progression can occur in the following order:

1. new (started by the client)
2. negotiating (optional)
3. authenticating (optional)
4. established
5. finishing (optional, started by the client)
6. finished OR failed (final)

In Go, session negotiation, authentication, and establishment are handled by
`client.Client` and `server.Server`.
You just need to make sure that the server and client are configured accordingly the desired behavior.

For instance, to require TLS for WebSocket connections, configure the server and client for WSS:

```go
server := serverpkg.NewServerBuilder().
    // Enable the TLS encryption option for all sessions
    EncryptionOptions(protocol.SessionEncryptionTLS).
    // Set up the secure WebSocket listener providing a certificate
    ListenWebsocket(addr, &serverpkg.WebsocketConfig{
        TLSConfig: &tls.Config{
            GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
                cert, err := tls.LoadX509KeyPair("cert.pem", "key.pem")
                if err != nil {
                    return nil, err
                }
                return &cert, nil
            },
        }}).
    // TODO: Setup other server options
    Build()
```

And on the client side, use a WSS URL and the TLS server name:

```go
client := limeclient.NewClientBuilder().
    Encryption(protocol.SessionEncryptionTLS).
    UseWebsocket("wss://localhost:55321", nil, &tls.Config{ServerName: "localhost"}).
    // TODO: Setup other client options
    Build()
```

You may also want to configure the server and client authentication mechanisms.
The Lime Go library supports the following schemes:

- Guest (no authentication)
- Plain (password)
- Key
- Transport (credentials delegated to the underlying transport)
- External (token emitted by an issuer)

To enable the use of plain authentication, in the server you should use the `EnablePlainAuthentication` method passing
the authentication handler function, like in the example below:

```go
server := serverpkg.NewServerBuilder().
    EnablePlainAuthentication(
        func(ctx context.Context, i protocol.Identity, pwd string) (*serverpkg.AuthenticationResult, error) {
        // TODO: implement checkCredentials to validate the user/password in your secret store
        if checkCredentials(i.Name, pwd) {
            return &serverpkg.AuthenticationResult{Role: serverpkg.DomainRoleMember}, nil
        }
        return &serverpkg.AuthenticationResult{Role: serverpkg.DomainRoleUnknown}, nil
    }).
    // TODO: Setup other server options
    Build()
```

On the client side, you can use the `PlainAuthentication` method to set the password that should be used:

```go
client := limeclient.NewClientBuilder().
    // Sets the identity name and password
    Name("john").
    PlainAuthentication("mysecretpassword").
    // TODO: Setup other client options
    Build()
```
