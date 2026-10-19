package inprocess

import (
	"context"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/takenet/lime-go/protocol"
	"go.uber.org/goleak"
)

func silentClose(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}

func receiveTransport(t *testing.T, transports <-chan Transport) Transport {
	select {
	case received := <-transports:
		return received
	case <-time.After(5 * time.Second):
		t.Fatal("transport listener timeout")
		return nil
	}
}

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

func TestInProcessTransportConcurrentClose(t *testing.T) {
	client, server := newInProcessTransportPair(InProcessAddr(t.Name()), 1)
	start := make(chan struct{})
	results := make(chan error, 32)
	for range 16 {
		for _, transport := range []*inProcessTransport{client, server} {
			go func() {
				<-start
				results <- transport.Close()
			}()
		}
	}
	close(start)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 32 {
		select {
		case err := <-results:
			assert.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("concurrent close did not complete")
		}
	}
	assert.False(t, client.Connected())
	assert.False(t, server.Connected())
}

func TestInProcessTransportCloseUnblocksAllReceivers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, server := newInProcessTransportPair(InProcessAddr(t.Name()), 1)
		results := make(chan error, 8)
		for range 4 {
			for _, transport := range []*inProcessTransport{client, server} {
				go func() {
					_, err := transport.Receive(context.Background())
					results <- err
				}()
			}
		}
		synctest.Wait()
		assert.NoError(t, client.Close())
		synctest.Wait()
		for range 8 {
			assert.Error(t, <-results)
		}
	})
}

func TestInProcessTransportSendSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	client := createClientInProcessTransport(t, addr)
	s := &protocol.Session{State: protocol.SessionStateNew}

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
	s := &protocol.Session{State: protocol.SessionStateNew}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancelFunc()
	if err := client.Send(ctx, s); err != nil {
		t.Fatal(err)
	}

	// Act
	e, err := server.Receive(ctx)

	// Assert
	assert.NoError(t, err)
	received, ok := e.(*protocol.Session)
	assert.True(t, ok)
	assert.Equal(t, s, received)
}

func TestInProcessTransportDrainsSessionBeforeClosed(t *testing.T) {
	client, server := newInProcessTransportPair("drain-final-session", 1)
	session := &protocol.Session{State: protocol.SessionStateFinished}
	if err := server.Send(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	envelope, err := client.Receive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if envelope != session {
		t.Fatal("final session was discarded")
	}
	if _, err := client.Receive(context.Background()); err == nil {
		t.Fatal("closed empty transport accepted receive")
	}
}

func TestInProcessTransportCompression(t *testing.T) {
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	transport := createClientInProcessTransport(t, addr)
	defer silentClose(transport)

	// Test SupportedCompression
	supported := transport.SupportedCompression()
	assert.NotNil(t, supported)
	assert.Contains(t, supported, protocol.SessionCompressionNone)

	// Test Compression
	compression := transport.Compression()
	assert.Equal(t, protocol.SessionCompressionNone, compression)

	// Test SetCompression - should return error as not supported
	err := transport.SetCompression(context.Background(), protocol.SessionCompressionGzip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestInProcessTransportEncryption(t *testing.T) {
	var addr InProcessAddr = "localhost"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	transport := createClientInProcessTransport(t, addr)
	defer silentClose(transport)

	// Test SupportedEncryption
	supported := transport.SupportedEncryption()
	assert.NotNil(t, supported)
	assert.Contains(t, supported, protocol.SessionEncryptionNone)

	// Test Encryption
	encryption := transport.Encryption()
	assert.Equal(t, protocol.SessionEncryptionNone, encryption)

	// Test SetEncryption - should return error as not supported
	err := transport.SetEncryption(context.Background(), protocol.SessionEncryptionTLS)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestInProcessTransportLocalAddr(t *testing.T) {
	var addr InProcessAddr = "test-local-addr"
	listener := createInProcessListener(t, addr, nil)
	defer silentClose(listener)
	transport := createClientInProcessTransport(t, addr)
	defer silentClose(transport)

	localAddr := transport.LocalAddr()
	assert.NotNil(t, localAddr)
	// Local addr is auto-generated for client
	assert.IsType(t, InProcessAddr(""), localAddr)
}

func TestInProcessTransportRemoteAddr(t *testing.T) {
	var addr InProcessAddr = "test-remote-addr"
	var transportChan = make(chan Transport, 1)
	listener := createInProcessListener(t, addr, transportChan)
	defer silentClose(listener)
	client := createClientInProcessTransport(t, addr)
	defer silentClose(client)
	server := receiveTransport(t, transportChan)
	defer silentClose(server)

	// From client's perspective, remote is the server address
	remoteAddr := client.RemoteAddr()
	assert.NotNil(t, remoteAddr)
	assert.Equal(t, addr, remoteAddr)
}

func TestInProcessAddrNetwork(t *testing.T) {
	addr := InProcessAddr("test-addr")

	network := addr.Network()
	assert.Equal(t, "in.process", network)
}
