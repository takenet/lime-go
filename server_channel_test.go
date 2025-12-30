package lime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sync/errgroup"
)

const errUnexpectedEnvelopeType = "unexpected envelope type"

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
	var wg errgroup.Group

	// Act
	wg.Go(func() error {
		err := client.Send(ctx, &Session{
			State: SessionStateNew,
		})
		if err != nil {
			return err
		}
		env, err := client.Receive(ctx)
		if err != nil {
			return err
		}
		s, ok := env.(*Session)
		if !ok {
			return errors.New(errUnexpectedEnvelopeType)
		}

		return client.Send(ctx, &Session{
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
	var wg errgroup.Group

	// Act
	wg.Go(func() error {
		e, err := client.Receive(ctx)
		if err != nil {
			errChan <- err
			return err
		}
		s, ok := e.(*Session)
		if !ok {
			err := errors.New(errUnexpectedEnvelopeType)
			errChan <- err
			return err
		}
		sessionChan <- s
		return nil
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
	var wg errgroup.Group

	// Act
	wg.Go(func() error {
		e, err := client.Receive(ctx)
		if err != nil {
			errChan <- err
			return err
		}
		s, ok := e.(*Session)
		if !ok {
			err := errors.New(errUnexpectedEnvelopeType)
			errChan <- err
			return err
		}
		sessionChan <- s
		return nil
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
