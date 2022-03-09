package lime

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
	"net"
	"testing"
	"time"
)

func createServer(listeners ...BoundListener) *Server {
	config := NewServerConfig()
	mux := &EnvelopeMux{}
	return NewServer(config, mux, listeners)
}

func createBoundInProcTransportListener(addr InProcessAddr) BoundListener {
	return BoundListener{
		Listener: NewInProcessTransportListener(addr),
		Addr:     addr,
	}
}

func createBoundTCPTransportListener(addr net.Addr) BoundListener {
	return BoundListener{
		Listener: NewTCPTransportListener(nil),
		Addr:     addr,
	}
}

func createBoundWSTransportListener(addr net.Addr) BoundListener {
	return BoundListener{
		Listener: NewWebsocketTransportListener(nil),
		Addr:     addr,
	}
}

func TestServer_ListenAndServe(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	eg, _ := errgroup.WithContext(context.Background())
	addr1 := InProcessAddr("localhost")
	listener1 := createBoundInProcTransportListener(addr1)
	addr2 := createLocalhostTCPAddress()
	listener2 := createBoundTCPTransportListener(addr2)
	addr3 := createLocalhostWSAddr()
	listener3 := createBoundWSTransportListener(addr3)
	srv := createServer(listener1, listener2, listener3)
	done := make(chan bool)

	// Act
	eg.Go(func() error {
		close(done)
		return srv.ListenAndServe()
	})
	<-done
	time.Sleep(16 * time.Millisecond)

	// Assert
	client1, err := DialInProcess(addr1, 1)
	assert.NoError(t, err)
	defer silentClose(client1)
	ses := &Session{State: SessionStateNew}
	err = client1.Send(ctx, ses)
	assert.NoError(t, err)
	client2, err := DialTcp(ctx, addr2, nil)
	assert.NoError(t, err)
	defer silentClose(client2)
	err = client2.Send(ctx, ses)
	assert.NoError(t, err)
	client3, err := DialWebsocket(ctx, fmt.Sprintf("ws://%s", addr3), nil, nil)
	assert.NoError(t, err)
	err = client3.Send(ctx, ses)
	defer silentClose(client3)
	assert.NoError(t, err)
	err = srv.Close()
	assert.NoError(t, err)
	assert.Error(t, eg.Wait(), ErrServerClosed)
}

func TestServerBuilder_Build(t *testing.T) {

}
