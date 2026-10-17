package channel

import (
	"io"

	"github.com/takenet/lime-go/internal/inprocess"
)

const (
	testDomain        = "limeprotocol.org"
	testExampleDomain = "example.com"
	testCommandID     = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	testSessionID     = "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	testHelloWorld    = "Hello world"
)

func createMessage() *Message {
	message := &Message{Envelope: Envelope{ID: testCommandID}}
	message.SetTo(Node{Identity: Identity{Name: "golang", Domain: testDomain}, Instance: "default"})
	message.SetContent(TextDocument(testHelloWorld))
	return message
}

func createNotification() *Notification {
	return &Notification{
		Envelope: Envelope{
			ID: testCommandID,
			To: Node{Identity: Identity{Name: "golang", Domain: testDomain}, Instance: "default"},
		},
		Event: NotificationEventReceived,
	}
}

func createGetPingCommand() *RequestCommand {
	command := &RequestCommand{Command: Command{
		Envelope: Envelope{
			ID: testCommandID,
			To: Node{Identity: Identity{Name: "postmaster", Domain: testDomain}},
		},
		Method: CommandMethodGet,
	}}
	command.SetURIString("/ping")
	return command
}

func createResponseCommand() *ResponseCommand {
	return &ResponseCommand{
		Command: Command{
			Envelope: Envelope{
				ID:   testCommandID,
				From: Node{Identity: Identity{Name: "postmaster", Domain: testDomain}},
			},
			Method: CommandMethodGet,
		},
		Status: CommandStatusSuccess,
	}
}

func newInProcessTransportPair(addr string, bufferSize int) (Transport, Transport) {
	return inprocess.NewPair(inprocess.InProcessAddr(addr), bufferSize)
}

func silentClose(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
