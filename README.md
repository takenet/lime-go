
LIME - A lightweight messaging library  
================================

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
			EnvelopeBase: lime.EnvelopeBase{ID: msg.ID, To: msg.From},
			Type:         msg.Type,
			Content:      msg.Content,
		})
	}

	// Build a server, listening for TCP connections in the 55321 port
	server := lime.NewServerBuilder().
		MessagesHandlerFunc(msgHandler).
		ListenTCP(net.TCPAddr{Port: 55321}, &lime.TCPConfig{}).
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
		log.Printf("Message received - Type: %v - Content: %v\n", msg.Type, msg.Content)
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
		Content: lime.PlainDocument("Hello world!"),
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

For instance, a message with a **text document** can be represented like this:

```json
{
  "id": "1",
  "to": "someone@domain.com",
  "type": "text/plain",
  "content": "Hello from Lime!"
}
```

In Go, the message envelope is implemented by the `lime.Message` type:

```go
msg := &lime.Message{}
msg.SetContent(lime.PlainDocument("Hello from Lime!")).
    SetID("1").
    SetToString("someone@domain.com")
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
[XMPP's Jabber ID](https://xmpp.org/rfcs/rfc3920.html#rfc.section.3).
But the _domain_ and _instance_ portions of the node are optional.

In this example, the content is a simple text but a message can be used to transport any type of document that can be 
represented as JSON.

For instance, to send a generic JSON document you can use the `application/json` type:

```json
{
  "id": "1",
  "to": "someone@domain.com",
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
    "timestamp": "2022-03-23T00:00:00.000Z",
    }).
    SetID("1").
    SetToString("someone@domain.com")
```


You can also can (and probably should) use custom MIME types for representing well-known types from your application
domain:

```json
{
  "id": "1",
  "to": "someone@domain.com",
  "type": "application/x-myapplication-person+json",
  "content": {
    "name": "John Doe",
    "address": "123 Main St",
    "online": true
  }
}
```

Using custom MIME types enables the mapping of types from your code. 
For that, these types need to implement the `Document` interface.


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

To send a message to someone, you can use the `SendMessage` method that is implemented both by the `lime.Server`
and `lime.Client` types:

```go
msg := &lime.Message{}
msg.SetContent(lime.TextDocument("Hello from Lime!")).
    SetID("1").
    SetToString("someone@domain.com")

err := client.SendMessage(context.Background(), msg)
```

And for receiving messages, you can use a handler that can be registered during the instantiation of the client or the
server:

```go
client := lime.NewClientBuilder().
    MessagesHandlerFunc(
        func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
            if txt, ok := msg.Content.(TextDocument); ok {
                fmt.Printf("Text message received - ID: %v - Type: %v - Content: %v\n", msg.ID, msg.Type, txt)	
            }
            return nil
        }).
	Build()
```

### Notification

A notification provides information about a message to its sender.
They are sent only for messages that have the `id` value defined.

For instance, a node can notify the that a message was received like this:

```json
{
  "id": "1",
  "from": "someone@domain.com",
  "to": "originator@domain.com",
  "event": "received"
}
```

In Go, you can use the `Notification(event)` method from the `*lime.Message` type for building a notification for a message:

```go
// Creates a corresponding notification to the message
if msg.ID != "" {
    not := msg.Notification(lime.NotificationEventReceived)
    // Send the notification 
    err := s.SendNotification(ctx, not)
}
```

Notifications can be sent by intermediates that are handling the message (like the server) or by the destination of the
message itself.

The protocol define the following notification events:
- **accepted**: The message was received and accepted by an intermediate.
- **dispatched**: The message was dispatched to the destination by the intermediate.
- **received**: The message was received by its destination.
- **consumed**: The message was processed (read) by its destination.
- **failed**: A problem occurred during the processing of the message.

A single message can generate multiple notifications, one for each step on its path to the destination.
By convention, the **consumed** and **failed** notifications are considered final, so no other notification should be
received by the sender after one of these.

In case of failed notifications, a **reason** value should be present in the notification. 

For instance, a server (intermediate) should notify the sender if it is unable to determine the destination session of 
a message:

```json
{
  "id": "1",
  "from": "server@domain.com",
  "to": "originator@domain.com",
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


