package lime

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestChannel_Established_WhenEstablished(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)

	// Act
	established := c.Established()

	// Assert
	assert.True(t, established)
}

func TestChannel_Established_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannel_Established_WhenTransportClosed(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	_ = client.Close()

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannel_SendMessage_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

func BenchmarkChannel_SendMessage(b *testing.B) {
	// Arrange
	count := b.N
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	messages := make([]*Message, count)
	for i := 0; i < count; i++ {
		messages[i] = createMessage()
	}
	errchan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < count; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
		}
		done <- true
	}()
	for _, m := range messages {
		_ = c.SendMessage(ctx, m)
	}
	select {
	case err := <-errchan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannel_SendMessage_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 0)
	c.setState(SessionStateEstablished)
	m1 := createMessage() // Will wait in the transport chan
	m2 := createMessage() // Will not be sent

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendMessage(ctx, m1)

	// Act
	err := c.SendMessage(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual)
}

func TestChannel_SendMessage_FullBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	m1 := createMessage() // Will wait in the transport chan
	m2 := createMessage() // Will wait in the channel buffer
	m3 := createMessage() // Will not be sent
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendMessage(ctx, m1)
	_ = c.SendMessage(ctx, m2)

	// Act
	err := c.SendMessage(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	var m *Message = nil
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendMessage(ctx, m)
	}, "send message: envelope cannot be nil")
}

func TestChannel_SendMessage_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.setState(state)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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

func BenchmarkChannel_SendNotification(b *testing.B) {
	// Arrange
	count := b.N
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	notifications := make([]*Notification, count)
	for i := 0; i < count; i++ {
		notifications[i] = createNotification()
	}
	errchan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < count; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
		}
		done <- true
	}()
	for _, n := range notifications {
		_ = c.SendNotification(ctx, n)
	}
	select {
	case err := <-errchan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannel_SendNotification_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 0)
	c.setState(SessionStateEstablished)
	m1 := createNotification() // Will wait in the transport chan
	m2 := createNotification() // Will not be sent

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendNotification(ctx, m1)

	// Act
	err := c.SendNotification(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual)
}

func TestChannel_SendNotification_FullBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	m1 := createNotification() // Will wait in the transport chan
	m2 := createNotification() // Will wait in the channel buffer
	m3 := createNotification() // Will not be sent
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendNotification(ctx, m1)
	_ = c.SendNotification(ctx, m2)

	// Act
	err := c.SendNotification(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	var n *Notification = nil
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendNotification(ctx, n)
	}, "send notification: envelope cannot be nil")
}

func TestChannel_SendNotification_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
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
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.setState(state)
	}()
	actual, err := c.ReceiveNotification(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive notification: channel closed", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_SendCommand_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, cmd, actual)
}

func TestChannel_SendCommand_Batch(t *testing.T) {
	// Arrange
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	commands := make([]*Command, count)
	for i := 0; i < count; i++ {
		commands[i] = createGetPingCommand()
	}
	var actuals []*Command
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
			cmd, ok := e.(*Command)
			if !ok {
				errchan <- errors.New("unexpected envelope type")
				return
			}
			actuals = append(actuals, cmd)
		}
		done <- true
	}()
	for _, cmd := range commands {
		err := c.SendCommand(ctx, cmd)
		assert.NoError(t, err)
	}
	select {
	case err := <-errchan:
		t.Fatal(err)
	case <-done:
		break
	}

	// Assert
	assert.Equal(t, commands, actuals)
}

func BenchmarkChannel_SendCommand(b *testing.B) {
	// Arrange
	count := b.N
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	commands := make([]*Command, count)
	for i := 0; i < count; i++ {
		commands[i] = createGetPingCommand()
	}
	errchan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < count; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
		}
		done <- true
	}()
	for _, cmd := range commands {
		_ = c.SendCommand(ctx, cmd)
	}
	select {
	case err := <-errchan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannel_SendCommand_NoBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 0)
	c.setState(SessionStateEstablished)
	m1 := createGetPingCommand() // Will wait in the transport chan
	m2 := createGetPingCommand() // Will not be sent

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendCommand(ctx, m1)

	// Act
	err := c.SendCommand(ctx, m2)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send command: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual)
}

func TestChannel_SendCommand_FullBuffer(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	m1 := createGetPingCommand() // Will wait in the transport chan
	m2 := createGetPingCommand() // Will wait in the channel buffer
	m3 := createGetPingCommand() // Will not be sent
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	_ = c.SendCommand(ctx, m1)
	_ = c.SendCommand(ctx, m2)

	// Act
	err := c.SendCommand(ctx, m3)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send command: context deadline exceeded", err.Error())
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	actual1, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m1, actual1)
	actual2, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m2, actual2)
}

func TestChannel_SendCommand_NilCommand(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	var cmd *Command = nil
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendCommand(ctx, cmd)
	}, "send command: envelope cannot be nil")
}

func TestChannel_SendCommand_WhenNew(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendCommand(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send command: cannot do in the new state", err.Error())
}

func TestChannel_ReceiveCommand_WhenEstablished(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = server.Send(ctx, cmd)

	// Act
	actual, err := c.ReceiveCommand(ctx)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, cmd, actual)
}

func TestChannel_ReceiveCommand_WhenContextCanceled(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// Act
	actual, err := c.ReceiveCommand(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive command: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_ReceiveCommand_WhenFinishedState(t *testing.T) {
	receiveCommandWithState(t, SessionStateFinished)
}

func TestChannel_ReceiveCommand_WhenFailedState(t *testing.T) {
	receiveCommandWithState(t, SessionStateFailed)
}

func receiveCommandWithState(t *testing.T, state SessionState) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.setState(state)
	}()
	actual, err := c.ReceiveCommand(ctx)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "receive command: channel closed", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_ProcessCommand(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	respCmd := createResponseCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go func() {
		_, err := server.Receive(ctx)
		if err != nil {
			cancel()
			return
		}

		_ = server.Send(ctx, respCmd)
	}()

	// Act
	actual, err := c.ProcessCommand(ctx, reqCmd)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, respCmd, actual)
}

func TestChannel_ProcessCommand_WhenContextCancelled(t *testing.T) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Act
	actual, err := c.ProcessCommand(ctx, reqCmd)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "process command: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
}

func TestChannel_ProcessCommand_ResponseWithAnotherId(t *testing.T) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	respCmd := createResponseCommand()
	respCmd.ID = "other-id"
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	go func() {
		_, err := server.Receive(ctx)
		if err != nil {
			cancel()
			return
		}

		_ = server.Send(ctx, respCmd)
	}()

	// Act
	actual, err := c.ProcessCommand(ctx, reqCmd)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "process command: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
	ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	actualRespCmd, err := c.ReceiveCommand(ctx)
	assert.NoError(t, err)
	assert.Equal(t, respCmd, actualRespCmd)
}
