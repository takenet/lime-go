package lime

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func createChannel(t *testing.T, tran Transport, bufferSize int) *channel {
	c, err := newChannel(tran, bufferSize)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChannel_Established_WhenEstablished(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)

	// Act
	established := c.Established()

	// Assert
	assert.True(t, established)
}

func TestChannel_Established_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannel_Established_WhenTransportClosed(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	_ = client.Close()

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannel_SendMessage_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
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

func TestChannel_SendMessage_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := createChannel(t, client, 0)
	_ = c.setState(SessionStateEstablished)
	m1 := createMessage() // Will wait in the transport chan
	m2 := createMessage() // Will not be sent

	ctx, cancelFunc := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendMessage(ctx, m1)

	// Act
	err := c.SendMessage(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancelFunc()
	ctx, cancelFunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual)
}

func TestChannel_SendMessage_FullBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	m1 := createMessage() // Will wait in the transport chan
	m2 := createMessage() // Will wait in the channel buffer
	m3 := createMessage() // Will not be sent
	ctx, cancelFunc := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendMessage(ctx, m1)
	_ = c.SendMessage(ctx, m2)

	// Act
	err := c.SendMessage(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancelFunc()
	ctx, cancelFunc = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	actual1, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual1)
	actual2, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m2, actual2)
}

func TestChannel_SendMessage_NilMessage(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	var m *Message = nil
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendMessage(ctx, m)
	}, "send message: envelope cannot be nil")
}

func TestChannel_SendMessage_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	m := createMessage()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: cannot do in the new state", err.Error())
}

func TestChannel_ReceiveMessage_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	_ = server.Send(ctx, m)

	// Act
	actual, err := c.ReceiveMessage(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, m, actual)
}

func TestChannel_ReceiveMessage_WhenContextCanceled(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancelFunc()

	// Act
	actual, err := c.ReceiveMessage(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive message: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_ReceiveMessage_WhenFinishedState(t *testing.T) {
	receiveMessageWithState(t, SessionStateFinished)
}

func TestChannel_ReceiveMessage_WhenFailedState(t *testing.T) {
	receiveMessageWithState(t, SessionStateFailed)
}

func receiveMessageWithState(t *testing.T, state SessionState) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()

	// Act
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = c.setState(state)
	}()
	actual, err := c.ReceiveMessage(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive message: channel closed", err.Error())
	assert.Nil(t, actual)
}
