package lime

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"time"
)

// Wrapper to simulate Go 1.25 sync.WaitGroup.Go feature
type WaitGroup struct {
	sync.WaitGroup
}

func (wg *WaitGroup) Go(f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}

func TestServerChannelEstablishSessionWhenGuest(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := testSessionID
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: "server1",
	}
	c := NewServerChannel(server, 1, serverNode, sessionID)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	clientNode := Node{
		Identity: Identity{Name: "golang", Domain: testDomain},
		Instance: "home",
	}
	var wg WaitGroup

	// Act
	// Use sync.WaitGroup.Go (1.25) pattern
	wg.Go(func() {
		err := client.Send(ctx, &Session{
			State: SessionStateNew,
		})
		if err != nil {
			return
		}
		env, err := client.Receive(ctx)
		if err != nil {
			return
		}
		s, ok := env.(*Session)
		if !ok {
			return
		}

		_ = client.Send(ctx, &Session{
			Envelope:       Envelope{ID: s.ID, From: clientNode},
			State:          SessionStateAuthenticating,
			Scheme:         AuthenticationSchemeGuest,
			Authentication: &GuestAuthentication{},
		})
	})

	err := c.EstablishSession(
		ctx,
		[]SessionCompression{SessionCompressionNone},
		[]SessionEncryption{SessionEncryptionTLS},
		[]AuthenticationScheme{AuthenticationSchemeGuest},
		func(context.Context, Identity, Authentication) (*AuthenticationResult, error) {
			return &AuthenticationResult{Role: DomainRoleMember}, nil
		},
		func(context.Context, Node, *ServerChannel) (Node, error) {
			return clientNode, nil
		},
	)

	wg.Wait() // Ensure goroutine finishes (though EstablishSession is the main blocker usually)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, serverNode, c.LocalNode())
	assert.Equal(t, clientNode, c.RemoteNode())
	assert.Equal(t, SessionStateEstablished, c.state)
	assert.True(t, c.Established())
	assert.True(t, c.transport.Connected())
}

func TestServerChannelFinishSession(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := testSessionID
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: "server1",
	}
	c := NewServerChannel(server, 1, serverNode, sessionID)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.setState(SessionStateEstablished)
	sessionChan := make(chan *Session)
	errChan := make(chan error)
	var wg WaitGroup

	// Act
	wg.Go(func() {
		e, err := client.Receive(ctx)
		if err != nil {
			errChan <- err
			return
		}
		s, ok := e.(*Session)
		if !ok {
			errChan <- errors.New("unexpected envelope type")
			return
		}
		sessionChan <- s
	})

	time.Sleep(5 * time.Millisecond)
	err := c.FinishSession(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionStateFinished, c.state)
	assert.False(t, c.Established())
	assert.False(t, c.transport.Connected())
	var s *Session
	select {
	case <-ctx.Done():
		assert.FailNow(t, ctx.Err().Error())
	case err := <-errChan:
		assert.FailNow(t, err.Error())
	case s = <-sessionChan:
		break
	}
	assert.Equal(t, sessionID, s.ID)
	assert.Equal(t, serverNode, s.From)
	assert.Equal(t, SessionStateFinished, s.State)
	wg.Wait()
}

func TestServerChannelFailSession(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := testSessionID
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: testDomain},
		Instance: "server1",
	}
	c := NewServerChannel(server, 1, serverNode, sessionID)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	c.setState(SessionStateEstablished)
	r := &Reason{
		Code:        1,
		Description: "The session has failed",
	}
	sessionChan := make(chan *Session)
	errChan := make(chan error)
	var wg WaitGroup

	// Act
	wg.Go(func() {
		e, err := client.Receive(ctx)
		if err != nil {
			errChan <- err
			return
		}
		s, ok := e.(*Session)
		if !ok {
			errChan <- errors.New("unexpected envelope type")
			return
		}
		sessionChan <- s
	})

	time.Sleep(5 * time.Millisecond)
	err := c.FailSession(ctx, r)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, SessionStateFailed, c.state)
	assert.False(t, c.Established())
	assert.False(t, c.transport.Connected())
	var s *Session
	select {
	case <-ctx.Done():
		assert.FailNow(t, ctx.Err().Error())
	case err := <-errChan:
		assert.FailNow(t, err.Error())
	case s = <-sessionChan:
		break
	}
	assert.Equal(t, sessionID, s.ID)
	assert.Equal(t, serverNode, s.From)
	assert.Equal(t, SessionStateFailed, s.State)
	assert.Equal(t, r, s.Reason)
	wg.Wait()
}
