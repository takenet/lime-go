package lime

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func createChannel(t *testing.T, tran Transport) *channel {
	c, err := newChannel(tran, 1)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChannel_SendMessage_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client)
	c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m, actual)
}

func TestChannel_SendMessage_NilMessage(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client)
	c.setState(SessionStateEstablished)
	var m *Message = nil
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.Error(t, err)
}

func TestChannel_SendMessage_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client)
	m := createMessage()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "cannot send in the new state", err.Error())
}
