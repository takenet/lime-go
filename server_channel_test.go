package lime

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestServerChannel_EstablishSession_WhenGuest(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "server1",
	}
	c := NewServerChannel(server, 1, serverNode, sessionID)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	clientNode := Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "home",
	}

	// Act
	go func() {
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
			EnvelopeBase:   EnvelopeBase{ID: s.ID, From: clientNode},
			State:          SessionStateAuthenticating,
			Scheme:         AuthenticationSchemeGuest,
			Authentication: &GuestAuthentication{},
		})
	}()
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

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, serverNode, c.LocalNode())
	assert.Equal(t, clientNode, c.RemoteNode())
	assert.Equal(t, SessionStateEstablished, c.state)
	assert.True(t, c.Established())
	assert.True(t, c.transport.Connected())
}

func TestServerChannel_FinishSession(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "server1",
	}
	c := NewServerChannel(server, 1, serverNode, sessionID)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c.setState(SessionStateEstablished)
	sessionChan := make(chan *Session)
	errChan := make(chan error)

	// Act
	go func() {
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
	}()
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
}

func TestServerChannel_FailSession(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
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

	// Act
	go func() {
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
	}()
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
}
