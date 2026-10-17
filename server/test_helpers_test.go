package server

import (
	"io"
	"net"

	"github.com/takenet/lime-go/internal/inprocess"
)

type InProcessAddr = inprocess.InProcessAddr

const (
	testDomain            = "limeprotocol.org"
	testExampleDomain     = "example.com"
	testCommandID         = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	testHelloWorld        = "Hello world"
	testPasswordPlainText = "correct-password"
	testKey               = "valid-key-123"
	testToken             = "valid-token"
	testTrustedIssuer     = "trusted-issuer"
)

func createMessage() *Message {
	message := &Message{}
	message.ID = testCommandID
	message.SetTo(Node{Identity: Identity{Name: "golang", Domain: testDomain}, Instance: "default"})
	document := TextDocument(testHelloWorld)
	message.SetContent(&document)
	return message
}

func NewInProcessTransportListener(addr InProcessAddr) TransportListener {
	return inprocess.NewInProcessTransportListener(addr)
}

func DialInProcess(addr InProcessAddr, bufferSize int) (Transport, error) {
	return inprocess.DialInProcess(addr, bufferSize)
}

func createLocalhostWSAddr() net.Addr {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	addr := listener.Addr()
	_ = listener.Close()
	return addr
}

func silentClose(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
