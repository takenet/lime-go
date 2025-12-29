package lime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func createInProcessListener(t *testing.T, addr InProcessAddr, transportChan chan Transport) TransportListener {
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

func createClientInProcessTransport(t *testing.T, addr InProcessAddr) Transport {
	client, err := DialInProcess(addr, 1)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	return client
}

func TestInProcessTransportDialWhenListening(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.NoError(t, err)
}

func TestInProcessTransportDialWhenNotListening(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransportDialAfterListenerClosed(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err := DialInProcess(addr, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransportDialOtherAddress(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	var addr2 InProcessAddr = "remote"

	// Act
	_, err := DialInProcess(addr2, 1)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refused")
}

func TestInProcessTransportCloseWhenOpen(t *testing.T) {
	// Arrange
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientInProcessTransport(t, addr)

	// Act
	err := client.Close()

	// Assert
	assert.NoError(t, err)
}

func TestInProcessTransportSendSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientInProcessTransport(t, addr)
	s := createSession()

	// Act
	err := client.Send(context.Background(), s)

	// Assert
	assert.NoError(t, err)
}

func TestInProcessTransportReceiveSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	var addr InProcessAddr = "localhost"
	var transportChan = make(chan Transport, 1)
	listener := createInProcessListener(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientInProcessTransport(t, addr)
	server := receiveTransport(t, transportChan)
	s := createSession()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 250*time.Millisecond)
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
