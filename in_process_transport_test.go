package lime

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func createInProcessListener(addr InProcessAddr, transportChan chan Transport, t *testing.T) TransportListener {
	listener := NewInProcessTransportListener(addr)
	if err := listener.Listen(context.Background(), addr); err != nil {
		t.Fatal(err)
		return nil
	}

	if transportChan != nil {
		go func() {
			for {
				t, err := listener.Accept(context.Background())
				if err != nil {
					break
				}
				transportChan <- t
			}
		}()
	}

	return listener
}

func createClientInProcessTransport(addr InProcessAddr, t *testing.T) Transport {
	client, err := DialInProcess(addr, 1)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func TestInProcessTransport_Dial_WhenListening(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(addr, nil, t)
	defer listener.Close()

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.NoError(t, err)
}

func TestInProcessTransport_Dial_WhenNotListening(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransport_Dial_AfterListenerClosed(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(addr, nil, t)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransport_Dial_OtherAddress(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(addr, nil, t)
	defer listener.Close()
	var addr2 InProcessAddr = "remote"

	// Act
	_, err := DialInProcess(addr2, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransport_Close_WhenOpen(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(addr, nil, t)
	defer listener.Close()
	client := createClientInProcessTransport(addr, t)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestInProcessTransport_Send_Session(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(addr, nil, t)
	defer listener.Close()
	client := createClientInProcessTransport(addr, t)
	s := createSession()

	// Act
	err := client.Send(context.Background(), s)

	// Assert
	assert.NoError(t, err)
}
func TestInProcessTransport_Receive_Session(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	var transportChan = make(chan Transport, 1)
	listener := createInProcessListener(addr, transportChan, t)
	defer listener.Close()
	client := createClientInProcessTransport(addr, t)
	server := receiveTransport(t, transportChan)
	s := createSession()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	if err := client.Send(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.NoError(t, err)
	received, ok := e.(*Session)
	assert.True(t, ok)
	assert.Equal(t, s, received)
}
