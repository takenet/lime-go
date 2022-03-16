package lime

import (
	"context"
	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
	"testing"
	"time"
)

func TestClientChannel_EstablishSession_WhenStateEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := NewClientChannel(client, 1)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	clientNode := Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "home",
	}
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "server1",
	}

	// Act
	go func() {
		_, err := server.Receive(ctx)
		if err != nil {
			return
		}
		_ = server.Send(
			ctx,
			&Session{
				EnvelopeBase: EnvelopeBase{
					ID:   sessionID,
					From: serverNode,
					To:   clientNode,
				},
				State: SessionStateEstablished,
			})
	}()

	actual, err := c.EstablishSession(
		ctx,
		func(compressions []SessionCompression) SessionCompression {
			return compressions[0]
		},
		func(encryptions []SessionEncryption) SessionEncryption {
			return encryptions[0]
		},
		clientNode.Identity,
		func(schemes []AuthenticationScheme, authentication Authentication) Authentication {
			auth := GuestAuthentication{}
			return &auth
		},
		clientNode.Instance,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, sessionID, actual.ID)
	assert.Equal(t, serverNode, actual.From)
	assert.Equal(t, clientNode, actual.To)
	assert.Equal(t, SessionStateEstablished, actual.State)
	assert.Equal(t, serverNode, c.RemoteNode())
	assert.Equal(t, clientNode, c.LocalNode())
	assert.Equal(t, SessionStateEstablished, c.state)
	assert.True(t, c.Established())
	assert.True(t, c.transport.Connected())
}

func TestClientChannel_EstablishSession_WhenStateFailed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := NewClientChannel(client, 1)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	clientNode := Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "home",
	}
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "server1",
	}

	// Act
	go func() {
		_, err := server.Receive(ctx)
		if err != nil {
			return
		}
		_ = server.Send(
			ctx,
			&Session{
				EnvelopeBase: EnvelopeBase{
					ID:   sessionID,
					From: serverNode,
				},
				Reason: &Reason{
					Code:        1,
					Description: "Session failed",
				},
				State: SessionStateFailed,
			})
	}()

	actual, err := c.EstablishSession(
		ctx,
		func(compressions []SessionCompression) SessionCompression {
			return compressions[0]
		},
		func(encryptions []SessionEncryption) SessionEncryption {
			return encryptions[0]
		},
		clientNode.Identity,
		func(schemes []AuthenticationScheme, authentication Authentication) Authentication {
			auth := GuestAuthentication{}
			return &auth
		},
		clientNode.Instance,
	)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, sessionID, actual.ID)
	assert.Equal(t, serverNode, actual.From)
	assert.Zero(t, actual.To)
	assert.Equal(t, SessionStateFailed, actual.State)
	assert.NotNil(t, actual.Reason)
	assert.Equal(t, 1, actual.Reason.Code)
	assert.Equal(t, "Session failed", actual.Reason.Description)
	assert.Zero(t, c.RemoteNode())
	assert.Zero(t, c.LocalNode())
	assert.Equal(t, SessionStateFailed, c.state)
	assert.False(t, c.Established())
	assert.False(t, c.transport.Connected())
}

func TestClientChannel_FinishSession(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := NewClientChannel(client, 1)
	defer silentClose(c)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	clientNode := Node{
		Identity: Identity{Name: "golang", Domain: "limeprotocol.org"},
		Instance: "home",
	}
	sessionID := "52e59849-19a8-4b2d-86b7-3fa563cdb616"
	serverNode := Node{
		Identity: Identity{Name: "postmaster", Domain: "limeprotocol.org"},
		Instance: "server1",
	}
	go func() {
		_, _ = server.Receive(ctx)
		_ = server.Send(ctx, &Session{
			EnvelopeBase: EnvelopeBase{ID: sessionID, From: serverNode, To: clientNode},
			State:        SessionStateEstablished})
	}()
	_, err := c.EstablishSession(ctx, NoneCompressionSelector, NoneEncryptionSelector, Identity{}, GuestAuthenticator, "")
	assert.NoError(t, err)

	// Act
	go func() {
		_, err := server.Receive(ctx)
		if err != nil {
			return
		}
		_ = server.Send(
			ctx,
			&Session{
				EnvelopeBase: EnvelopeBase{
					ID:   sessionID,
					From: serverNode,
					To:   clientNode,
				},
				State: SessionStateFinished,
			})
	}()

	actual, err := c.FinishSession(ctx)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, actual)
	assert.Equal(t, sessionID, actual.ID)
	assert.Equal(t, serverNode, actual.From)
	assert.Equal(t, clientNode, actual.To)
	assert.Equal(t, SessionStateFinished, actual.State)
	assert.Equal(t, SessionStateFinished, c.state)
	assert.False(t, c.Established())
	assert.False(t, c.transport.Connected())
}
