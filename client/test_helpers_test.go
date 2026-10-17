package client

import (
	"io"

	"github.com/takenet/lime-go/internal/inprocess"
	"github.com/takenet/lime-go/protocol"
)

type InProcessAddr = inprocess.InProcessAddr

const (
	testDomain         = "limeprotocol.org"
	testExampleDomain  = "example.com"
	testCommandID      = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	testClientInstance = "test-instance"
	testHelloWorld     = "Hello world"
)

func createMessage() *Message {
	message := &Message{Envelope: protocol.Envelope{ID: testCommandID}}
	message.SetTo(Node{Identity: Identity{Name: "golang", Domain: testDomain}, Instance: "default"})
	document := TextDocument(testHelloWorld)
	message.SetContent(&document)
	return message
}

func silentClose(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
