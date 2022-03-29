
LIME - A lightweight messaging library  
======================================

![Go](https://github.com/takenet/lime-go/workflows/Go/badge.svg?branch=master)

LIME allows you to build scalable, real-time messaging applications using a JSON-based 
[open protocol](http://limeprotocol.org). 
It's **fully asynchronous** and support persistent transports like TCP or Websockets.

You can send and receive any type of document into the wire as long it can be represented as JSON or text (plain or 
encoded with base64) and it has a **MIME type** to allow the other party handle it in the right way.

The connected nodes can send receipts to the other parties to notify events about messages (for instance, a message was 
received or the content invalid or not supported).

Besides that, there's a **REST capable** command interface with verbs (*get, set and delete*) and resource identifiers 
(URIs) to allow rich messaging scenarios. 
You can use that to provide services like on-band account registration or instance-messaging resources, like presence or 
roster management.

Finally, it has built-in support for authentication, transport encryption and compression.

Getting started
-----

### Server

For creating a server and start receiving connections, you should use the `lime.Server` type, which can be build using 
the `lime.NewServerBuilder()` function.

At least one **transport listener** (TCP, WebSocket or in-process) should be configured. 
You also should **register handlers** for processing the received envelopes.

The example below show how to create a simple TCP server that echoes every received message to its originator:

```go

package main

import (
	"context"
	"github.com/takenet/lime-go"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Message handler that echoes all received messages to the originator
	msgHandler := func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
		return s.SendMessage(ctx, &lime.Message{
			Envelope: lime.Envelope{ID: msg.ID, To: msg.From},
			Type:     msg.Type,
			Content:  msg.Content,
		})
	}

	// Build a server, listening for TCP connections in the 55321 port
	server := lime.NewServerBuilder().
		MessagesHandlerFunc(msgHandler).
		ListenTCP(&net.TCPAddr{Port: 55321}, &lime.TCPConfig{}).
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
	if err := server.ListenAndServe(); err != lime.ErrServerClosed {
		log.Printf("listen: %v\n", err)
	}
}
```

### Client 

In the client side, you may use the `lime.Client` type, which can be built using the helper method 
`lime.NewClientBuilder`.

```go
package main

import (
	"context"
	"github.com/takenet/lime-go"
	"log"
	"net"
	"time"
)

func main() {
	done := make(chan bool)
	
	// Defines a simple handler function for printing  
	// the received messages to the stdout
	msgHandler := func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
		if txt, ok := msg.Content.(lime.TextDocument); ok {
			log.Printf("Text message received - ID: %v - Type: %v - Content: %v\n", msg.ID, msg.Type, txt)
		}
		close(done)
		return nil
	}
	
	// Initialize the client
	client := lime.NewClientBuilder().
		UseTCP(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 55321}, &lime.TCPConfig{}).
		MessagesHandlerFunc(msgHandler).
		Build()

	// Prepare a simple text message to be sent
	msg := &lime.Message{
		Type: lime.MediaTypeTextPlain(),
		Content: lime.TextDocument("Hello world!"),
	}

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

Protocol overview
------------------

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

In Go, the message envelope is implemented by the `lime.Message` type:

```go
msg := &lime.Message{}
msg.SetContent(lime.TextDocument("Hello from Lime!")).
    SetID("1").
    SetToString("john")
```

In this example, the document value is the `Hello from Lime!` text and its MIME type is `text/plain`. 

This message also have a `id` property with value `1`. 
The id used to **correlate notifications** about the message.
This means that the message destination and intermediates may send notifications about the message status, using the 
same id.
So, if you are interested to know if a message that was sent by you was delivered or not, you should put a value in the 
id property.

The `to` property specifies the destination address of the message, and it is used by the server to route the envelope 
to the correct destination.
The address format is called **node** and is presented in the `name@domain/instance` format, similar to the 
[XMPP's Jabber ID](https://xmpp.org/rfcs/rfc3920.html#rfc.section.3), but the _domain_ and _instance_ portions of the 
node are optional.

In this example, the content is a simple text but a message can be used to transport any type of document that can be 
represented as JSON.

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
msg := &lime.Message{}
msg.SetContent(&lime.JsonDocument{
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
For this to be possible, these types need to implement the `lime.Document` interface.

```go
type Image struct {
    Caption string `json:"caption,omitempty"`
    URL     string `json:"url,omitempty"`
}

func (f *Image) MediaType() lime.MediaType {
    return lime.MediaType{
        Type:    "application",
        Subtype: "x-app-image",
        Suffix:  "json",
    }
}

// To register your custom type, use the RegisterDocumentFactory function.
func init() {
    lime.RegisterDocumentFactory(func() Document {
        return &Image{}
    })
}
```

For instance, to send a message to the "john" addresss you can use the `SendMessage` method that is implemented both by 
the `lime.Server` and `lime.Client` types:

```go
msg := &lime.Message{}
msg.SetContent(lime.TextDocument("Hello from Lime!")).
    SetID("1").
    SetToString("john")

err := client.SendMessage(context.Background(), msg)
```

And for receiving messages, you can use a message handler that can be registered during the instantiation of the client
or the server:

```go
client := lime.NewClientBuilder().
    MessagesHandlerFunc(
        func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
            if txt, ok := msg.Content.(lime.TextDocument); ok {
                fmt.Printf("Text message received - ID: %v - Type: %v - Content: %v\n", msg.ID, msg.Type, txt)	
            }
            return nil
        }).
	Build()
```

### Notification

A notification provide information about a message to its sender.
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

In Go, you can use the `Notification(event)` method from the `*lime.Message` type for building a notification for the 
message:

```go
// Creates a corresponding notification to the message
if msg.ID != "" {
    not := msg.Notification(lime.NotificationEventReceived)
    // Send the notification 
    err := s.SendNotification(ctx, not)
}
```

Notifications can be sent by the **destination of the message or by intermediates** - like a server that routes the 
message.

The protocol define the following notification events:
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
not := msg.FailedNotification(&lime.Reason{Code: 1, Description: "Destination not found"})
```

### Command

The command envelope is used to **manipulate resources of a remote node**. 
It provides a REST capable interface, with a URI and methods (verbs), much like the HTTP protocol.
It also supports multiplexing, so the connection is not blocked when a request is sent.

A command can be a request - which haves the `uri` value - or a response - with the `status` value.

As example, you can use it for managing your contact list or to set your current status (available, busy, away).
Other common use is **the in-band registration**, where users can create Lime accounts in the protocol itself.

The advantage of using commands is that you can use the **same existing connection** that is used for messaging instead
of creating one or more out-of-band connections - like in HTTP for instance.
This is more efficient in terms of energy consumption but also is more performatic as well. 
The session is already established and authenticated, so it avoids the addition overhead of a TLS handshake and 
authentication that a new connection would require. 

But there is a limitation: the command interface only supports JSON payloads, so you should avoid use it for 
transmitting binary or any kind of large content.

Much like an HTTP service, the URI and methods that you may use in commands depends on what the server implements.

For instance, a server could implement a contact management service. 
In this example, you could be able to send a command like this:

```json
{
  "id": "2",
  "method": "get",
  "uri": "/contacts"
}
```

Semantically, this means that you want to retrieve all contacts that are stored in the server.
And the server may respond to this request with something like this: 

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

Note that the value of the `id` property is the same of the request.
This is how we know that a response is to a specific request, so it is important to avoid using duplicate ids to avoid
collisions. A way for doing this is to use GUID (UUID) values as id for the requests.

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

For creating a request command in Go, you can use the `lime.RequestCommand` type:

```go
reqCmd := &lime.RequestCommand{}
reqCmd.SetURIString("/contacts").
    SetMethod(lime.CommandMethodGet).
    SetID(lime.NewEnvelopeID())
```

Note that for the `id` value, we are using the value returned by the `lime.NewEnvelopeID()` function, which will return
a UUID v4 string (something like `3cdd2654-911d-497e-834a-3b7865510155`).

For sending a command request, you should use the `ProcessCommand` method from the `lime.Client` and `lime.Server` 
types, instead of the `SendCommand` method. The `ProcessCommand` method take care of await for the correspondent
response command.

```go
respCmd, err := client.ProcessCommand(context.Background(), reqCmd)
if err == nil {
    // TODO: Handle the response
}
```

In the server side, you can add handlers for specific commands using the `RequestCommandHandler*` methods from
the `lime.Server` type.

```go
server := lime.NewServerBuilder(). 	
    RequestCommandHandlerFunc(
        // Set a predicate for filtering only the get contacts commands
        func(cmd *lime.RequestCommand) bool {
            return cmd.Method == lime.CommandMethodGet && cmd.URI.Path() == "/contacts"
        },
        // The handler implementation
        func(ctx context.Context, cmd *lime.RequestCommand, s lime.Sender) error {
            // Create a document collection of contacts
            contacts := &lime.DocumentCollection{
                Total:    2,
                ItemType: chat.MediaTypeContact(),
                Items: []lime.Document{
                    &chat.Contact{Name: "John Doe"},
                    &chat.Contact{Name: "Mary"},
                },
            }
            // Send the response to the sender
            respCmd := cmd.SuccessResponseWithResource(contacts)
            return s.SendResponseCommand(ctx, respCmd)
        }).
    // TODO: Setup other server options
    Build()
```

This is also valid for the `lime.Client` type. 
In Lime, the **client can receive and process commands requests** from other nodes, like the server.

### Session

> Note: The session establishment flow is automatically handled by the library.

The session envelope is used for the negotiation, authentication and establishment of the communication channel between
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
The server should reply this with another session envelope, according to the session state that it wants to enforce.

For instance, the server may want to present the client the transport options for negotiation.
This is useful for applying the TLS encryption to an unencrypted connection (the TCP connection starts without 
encryption by default).

```json
{
  "id": "0676a702-a7d6-43e6-947f-bde3c3e25eb5",
  "from": "server@localhost/s1",
  "state": "negotiating",
  "compressionOptions": ["none"],
  "encryptionOptions": ["none", "tls"]
}
```

Note that this envelope haves a `id` defined, which is the **session id**. 
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

In Go, the session negotiation, authentication, and establishment process is **automatically handled** by the 
`lime.Client` and `lime.Server` types.
You just need to make sure that the server and client are configured accordingly the desired behavior.

For instance, if you want to ensure that the TCP transport connections are using the TLS encryption, you will need to
configure the server similarly to this:

```go
server := lime.NewServerBuilder().
    // Enable the TLS encryption option for all sessions
    EncryptionOptions(lime.SessionEncryptionTLS).
    // Set up the TCP listener providing a certificate
    ListenTCP(addr, &lime.TCPConfig{
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

And in the client side, you should set up the TLS encryption option and the TCP config:

```go
client := lime.NewClientBuilder().
    Encryption(lime.SessionEncryptionTLS).
    UseTCP(addr, &lime.TCPConfig{
        TLSConfig:   &tls.Config{ServerName: "localhost"},
    }).
    // TODO: Setup other client options
    Build()
```

You may also want to configure the server and client authentication mechanisms.
The Lime Go library supports the following schemes:
- Guest (no authentication)
- Plain (password)
- Key
- Transport (mutual TLS on TCP)
- External (token emitted by an issuer)

To enable the use of plain authentication, in the server you should use the `EnablePlainAuthentication` method passing
the authentication handler function, like in the example below:

```go
server := lime.NewServerBuilder().
    EnablePlainAuthentication(
        func(ctx context.Context, i lime.Identity, pwd string) (*lime.AuthenticationResult, error) {
        // TODO: implement checkCredentials to validate the user/password in your secret store
        if checkCredentials(i.Name, pwd) {
            return &lime.AuthenticationResult{Role: lime.DomainRoleMember}, nil
        }
        return &lime.AuthenticationResult{Role: lime.DomainRoleUnknown}, nil
    }).
    // TODO: Setup other server options
    Build()
```

On the client side, you can use the `PlainAuthentication` method to set the password that should be used:

```go
client := lime.NewClientBuilder().
    // Sets the identity name and password
    Name("john").
    PlainAuthentication("mysecretpassword").
    // TODO: Setup other client options
    Build()
```