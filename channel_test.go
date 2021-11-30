package lime

import (
	"context"
	"errors"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m, actual)
}

func TestChannel_SendMessage_Batch(t *testing.T) {
	// Arrange
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	messages := make([]*Message, count)
	for i := 0; i < count; i++ {
		messages[i] = createMessage()
	}
	var actuals []*Message
	errchan := make(chan error)
	done := make(chan bool)

	// Act
	go func() {
		for i := 0; i < count; i++ {
			e, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
			m, ok := e.(*Message)
			if !ok {
				errchan <- errors.New("unexpected envelope type")
				return
			}
			actuals = append(actuals, m)
		}
		done <- true
	}()
	for _, m := range messages {
		err := c.SendMessage(ctx, m)
		assert.NoError(t, err)
	}
	select {
	case err := <-errchan:
		t.Fatal(err)
	case <-done:
		break
	}

	// Assert
	assert.Equal(t, messages, actuals)
}

func TestChannel_SendMessage_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := createChannel(t, client, 0)
	_ = c.setState(SessionStateEstablished)
	m1 := createMessage() // Will wait in the transport chan
	m2 := createMessage() // Will not be sent

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendMessage(ctx, m1)

	// Act
	err := c.SendMessage(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendMessage(ctx, m1)
	_ = c.SendMessage(ctx, m2)

	// Act
	err := c.SendMessage(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

func TestChannel_SendNotification_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	err := c.SendNotification(ctx, n)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, n, actual)
}

func TestChannel_SendNotification_Batch(t *testing.T) {
	// Arrange
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	notifications := make([]*Notification, count)
	for i := 0; i < count; i++ {
		notifications[i] = createNotification()
	}
	var actuals []*Notification
	errchan := make(chan error)
	done := make(chan bool)

	// Act
	go func() {
		for i := 0; i < count; i++ {
			e, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
			n, ok := e.(*Notification)
			if !ok {
				errchan <- errors.New("unexpected envelope type")
				return
			}
			actuals = append(actuals, n)
		}
		done <- true
	}()
	for _, n := range notifications {
		err := c.SendNotification(ctx, n)
		assert.NoError(t, err)
	}
	select {
	case err := <-errchan:
		t.Fatal(err)
	case <-done:
		break
	}

	// Assert
	assert.Equal(t, notifications, actuals)
}

func TestChannel_SendNotification_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := createChannel(t, client, 0)
	_ = c.setState(SessionStateEstablished)
	m1 := createNotification() // Will wait in the transport chan
	m2 := createNotification() // Will not be sent

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendNotification(ctx, m1)

	// Act
	err := c.SendNotification(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual)
}

func TestChannel_SendNotification_FullBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	m1 := createNotification() // Will wait in the transport chan
	m2 := createNotification() // Will wait in the channel buffer
	m3 := createNotification() // Will not be sent
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_ = c.SendNotification(ctx, m1)
	_ = c.SendNotification(ctx, m2)

	// Act
	err := c.SendNotification(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actual1, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual1)
	actual2, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m2, actual2)
}

func TestChannel_SendNotification_NilNotification(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	var n *Notification = nil
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendNotification(ctx, n)
	}, "send notification: envelope cannot be nil")
}

func TestChannel_SendNotification_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	err := c.SendNotification(ctx, n)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: cannot do in the new state", err.Error())
}

func TestChannel_ReceiveNotification_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Send(ctx, n)

	// Act
	actual, err := c.ReceiveNotification(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, n, actual)
}

func TestChannel_ReceiveNotification_WhenContextCanceled(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// Act
	actual, err := c.ReceiveNotification(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive notification: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_ReceiveNotification_WhenFinishedState(t *testing.T) {
	receiveNotificationWithState(t, SessionStateFinished)
}

func TestChannel_ReceiveNotification_WhenFailedState(t *testing.T) {
	receiveNotificationWithState(t, SessionStateFailed)
}

func receiveNotificationWithState(t *testing.T, state SessionState) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := createChannel(t, client, 1)
	_ = c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = c.setState(state)
	}()
	actual, err := c.ReceiveNotification(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive notification: channel closed", err.Error())
	assert.Nil(t, actual)
}
