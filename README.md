
LIME - A lightweight messaging library  
================================

![Go](https://github.com/takenet/lime-go/workflows/Go/badge.svg?branch=master)

LIME allows you to build scalable, real-time messaging applications using a JSON-based [open protocol](http://limeprotocol.org). It's **fully asynchronous** and supports any persistent transport like TCP or Websockets.

You can send and receive any type of document into the wire as long it can be represented as JSON or text (plain or encoded with base64) and it has a **MIME type** to allow the other party handle it in the right way.

The connected nodes can send receipts to the other parties to notify events about messages (for instance, a message was received or the content invalid or not supported).

Besides that, there's a **REST capable** command interface with verbs (*get, set and delete*) and resource identifiers (URIs) to allow rich messaging scenarios. You can use that to provide services like on-band account registration or instance-messaging resources, like presence or roster management.

Finally, it has built-in support for authentication, transport encryption and compression.

Getting started
-----

### Server

For creating a server and start receiving connections, you should use the `lime.Server` type, which can be build using the `lime.NewServerBuilder()` function.

At least one **transport listener** (TCP, WebSocket or in process) should be configured. 
You also should **register handlers** for processing the received envelopes.

The example below show how to create a simple TCP server that echoes every received message to its originator:

```go

package main

import (
	"context"
	"github.com/takenet/lime-go"
	"log"
	"net"
)

func main() {
	// Message handler that echoes all received messages to the originator
	msgHandler := func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
		return s.SendMessage(ctx, &lime.Message{
			EnvelopeBase: lime.EnvelopeBase{ID: msg.ID, To: msg.From},
			Type:    msg.Type,
			Content: msg.Content,
		})
	}

	// Build a server, listening for TCP connections in the 55321 port
	server := lime.NewServerBuilder().
		MessagesHandlerFunc(msgHandler).
		ListenTCP(net.TCPAddr{Port: 55321}, &lime.TCPConfig{}).
		Build()

	defer func() {
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

In the client side, you may use the `lime.Client` type, which can be built using the helper method `lime.NewClientBuilder`.

```go
package main

import (
	"context"
	"fmt"
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
		fmt.Printf("Message received - Type: %v - Content: %v\n", msg.Type, msg.Content)
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

Implementation overview
-----------------------

The basic protocol data package is called **envelope**. As mentioned before, there are four types:

* **Message** - Transports content between nodes
* **Notification** - Notify about message events
* **Command** - Provides an interface for resource management
* **Session** - Used in the establishment of the communication channel

All envelope types share some properties, like the `id` - the envelope's unique identifier - and the `from` and `to` routing information. But the types also have distinct properties, which are helpful in the deserialization when a JSON object is received by the transport.

The `Transport` interface represents a persistent connection for sending and receiving envelopes.
Currently, the library provides the `tcpTransport`, `webSocketTransport` and `inProcessTransport` implementations.

When two nodes are connected to each other, a **session** can be established between them.
To help the management of the session state, the library defines the `channel` type, an abstraction of the session over the `Transport` instance.
The node that received the connection is the **server**, and the one connecting is the **client**.
There are specific implementations of the interface for the server (`ServerChannel` that implements the derived `ServerChannel` interface) and the client (`ClientChannel` that implements `ClientChannel`), each one providing specific functionality for each role in the connection.
The only difference between the client and the server is related to the session state management, where the server has full control of it. 
Besides that, they share the same set of functionality.
A server uses a `TransportListener` instance to listen for new transport connections. The library provides the `tcpTransportListener` for TCP servers implementation.

### Session establishment

The server is responsible for the establishment of the session and its parameters, like the `id` and node information (both local and remote). 
It can optionally negotiate transport options and authenticate the client using a supported scheme. 

The most relevant transport option is the encryption. The library support **TLS encryption** for the `TcpTransport` implementation, allowing both server and client authentication via certificates.

After the transport options negotiation, the server can request client authentication. 
The server presents to the client the available schemes and the client should provide the scheme specific authentication data and identify itself with an identity, which is presented as **name@domain** (like an e-mail). 
Usually the domain of the client identity is the same of the server if the client is using a local authentication scheme (username/password) but can be a stranger domain if the client is using transport authentication (TLS certificate).

When the server establishes the session, it assigns to the client a unique node identifier, in the format **name@domain/instance** similar to the Jabber ID in the XMPP protocol. 
This identifier is important for envelope routing in multi-party server connection scenarios.

### Exchanging envelopes

With an established session the nodes can exchange messages, notifications and commands until the server finishes the session. 
The `Client` defines methods to send and receive specific envelopes, like the `SendMessage` and `ReceiveMessage` for messages or `SendCommand` and `ReceiveCommand` for commands.

#### Routing

The protocol doesn't define explicitly how envelope routing should work during a session. 
The only thing defined is that if an originator does not provide the `to` property value, it means that the message is addressed to the immediate remote party; in the same way if a node has received an envelope without the `from` property value, it must assume that the envelope is originated by the remote party.

An originator can send an envelope addresses to any destination to the other party, and it may or may not accept it. 
But an originator should address an envelope to a node different of the remote party only if it trusts it for receiving these envelopes. 
A remote party can be trusted for that if it has presented a valid domain certificate during the session negotiation. 
In this case, this node can receive and send envelopes for any identity of the authenticated domain.
