package lime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

const unexpectedEnvelopeTypeError = "unexpected envelope type"

func TestChannelEstablishedWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)

	// Act
	established := c.Established()

	// Assert
	assert.True(t, established)
}

func TestChannelEstablishedWhenNew(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannelEstablishedWhenTransportClosed(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	_ = client.Close()

	// Act
	established := c.Established()

	// Assert
	assert.False(t, established)
}

func TestChannelSendMessageWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, m, actual)
}

func TestChannelSendMessageBatch(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	messages := make([]*Message, count)
	for i := range count {
		messages[i] = createMessage()
	}
	var actuals []*Message
	errchan := make(chan error)
	done := make(chan bool)

	// Act
	go func() {
		for range count {
			e, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
			m, ok := e.(*Message)
			if !ok {
				errchan <- errors.New(unexpectedEnvelopeTypeError)
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

func BenchmarkChannelSendMessage(b *testing.B) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 0)
	c := newChannel(client, 0)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	messages := make([]*Message, b.N)
	for i := range messages {
		messages[i] = createMessage()
	}
	errChan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errChan <- err
				return
			}
		}
		done <- true
	}()
	for _, m := range messages {
		_ = c.SendMessage(ctx, m)
	}
	select {
	case <-ctx.Done():
		b.Fatal(ctx.Err())
	case err := <-errChan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannelSendMessageNilMessage(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	var m *Message = nil
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendMessage(ctx, m)
	}, "send message: envelope cannot be nil")
}

func TestChannelSendMessageWhenNew(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendMessage(ctx, m)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send message: cannot do in the new state", err.Error())
}

func TestChannelReceiveMessageWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	m := createMessage()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = server.Send(ctx, m)

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.MsgChan():
		// Assert
		assert.True(t, ok)
		assert.Equal(t, m, actual)
	}
}

func TestChannelReceiveMessageWhenFinishedState(t *testing.T) {
	receiveMessageWithState(t, SessionStateFinished)
}

func TestChannelReceiveMessageWhenFailedState(t *testing.T) {
	receiveMessageWithState(t, SessionStateFailed)
}

func receiveMessageWithState(t *testing.T, state SessionState) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.setState(state)
	}()

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.MsgChan():
		// Assert
		assert.False(t, ok)
		assert.Nil(t, actual)
	}
}

func TestChannelSendNotificationWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendNotification(ctx, n)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, n, actual)
}

func TestChannelSendNotificationBatch(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	notifications := make([]*Notification, count)
	for i := range count {
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
				errchan <- errors.New(unexpectedEnvelopeTypeError)
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

func BenchmarkChannelSendNotification(b *testing.B) {
	// Arrange
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	notifications := make([]*Notification, b.N)
	for i := range notifications {
		notifications[i] = createNotification()
	}
	errchan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < b.N; i++ {
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
	case <-ctx.Done():
		b.Fatal(ctx.Err())
	case err := <-errchan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannelSendNotificationNilNotification(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	var n *Notification = nil
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendNotification(ctx, n)
	}, "send notification: envelope cannot be nil")
}

func TestChannelSendNotificationWhenNew(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendNotification(ctx, n)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send notification: cannot do in the new state", err.Error())
}

func TestChannelReceiveNotificationWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	n := createNotification()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = server.Send(ctx, n)

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.NotChan():
		// Assert
		assert.True(t, ok)
		assert.Equal(t, n, actual)
	}
}

func TestChannelReceiveNotificationWhenFinishedState(t *testing.T) {
	receiveNotificationWithState(t, SessionStateFinished)
	defer goleak.VerifyNone(t)
}

func TestChannelReceiveNotificationWhenFailedState(t *testing.T) {
	receiveNotificationWithState(t, SessionStateFailed)
	defer goleak.VerifyNone(t)
}

func receiveNotificationWithState(t *testing.T, state SessionState) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.setState(state)
	}()

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.NotChan():
		// Assert
		assert.False(t, ok)
		assert.Nil(t, actual)
	}
}

func TestChannelSendRequestCommandWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendRequestCommand(ctx, cmd)

	// Assert
	assert.NoError(t, err)
	actual, err := server.Receive(ctx)
	assert.NoError(t, err)
	assert.Equal(t, cmd, actual)
}

func TestChannelSendRequestCommandBatch(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	count := 100
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	commands := make([]*RequestCommand, count)
	for i := range count {
		commands[i] = createGetPingCommand()
	}
	var actuals []*RequestCommand
	errchan := make(chan error)
	done := make(chan bool)

	// Act
	go func() {
		for range count {
			e, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
			cmd, ok := e.(*RequestCommand)
			if !ok {
				errchan <- errors.New(unexpectedEnvelopeTypeError)
				return
			}
			actuals = append(actuals, cmd)
		}
		done <- true
	}()
	for _, cmd := range commands {
		err := c.SendRequestCommand(ctx, cmd)
		assert.NoError(t, err)
	}
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case err := <-errchan:
		t.Fatal(err)
	case <-done:
		break
	}

	// Assert
	assert.Equal(t, commands, actuals)
}

func BenchmarkChannelSendRequestCommand(b *testing.B) {
	// Arrange
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	commands := make([]*RequestCommand, b.N)
	for i := range commands {
		commands[i] = createGetPingCommand()
	}
	errchan := make(chan error)
	done := make(chan bool)
	b.ResetTimer()

	// Act
	go func() {
		for i := 0; i < b.N; i++ {
			_, err := server.Receive(ctx)
			if err != nil {
				errchan <- err
				return
			}
		}
		done <- true
	}()
	for _, cmd := range commands {
		_ = c.SendRequestCommand(ctx, cmd)
	}
	select {
	case <-ctx.Done():
		b.Fatal(ctx.Err())
	case err := <-errchan:
		b.Fatal(err)
	case <-done:
		break
	}
}

func TestChannelSendRequestCommandNilCommand(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	var cmd *RequestCommand = nil
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act / Assert
	assert.Panics(t, func() {
		_ = c.SendRequestCommand(ctx, cmd)
	}, "send request command: envelope cannot be nil")
}

func TestChannelSendCommandWhenNew(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	err := c.SendRequestCommand(ctx, cmd)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "send request command: cannot do in the new state", err.Error())
}

func TestChannelReceiveCommandWhenEstablished(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	cmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	_ = server.Send(ctx, cmd)

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.ReqCmdChan():
		// Assert
		assert.True(t, ok)
		assert.Equal(t, cmd, actual)
	}
}

func TestChannelReceiveCommandWhenFinishedState(t *testing.T) {
	receiveCommandWithState(t, SessionStateFinished)
	defer goleak.VerifyNone(t)
}

func TestChannelReceiveCommandWhenFailedState(t *testing.T) {
	receiveCommandWithState(t, SessionStateFailed)
	defer goleak.VerifyNone(t)
}

func receiveCommandWithState(t *testing.T, state SessionState) {
	// Arrange
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.setState(state)
	}()

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actual, ok := <-c.ReqCmdChan():
		// Assert
		assert.False(t, ok)
		assert.Nil(t, actual)
	}
}

func TestChannelProcessCommand(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	respCmd := createResponseCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
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

func TestChannelProcessCommandWhenContextCanceled(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, _ := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	actual, err := c.ProcessCommand(ctx, reqCmd)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "process command: context deadline exceeded", err.Error())
	assert.Nil(t, actual)
}

func TestChannelProcessCommandResponseWithAnotherId(t *testing.T) {
	// Arrange
	defer goleak.VerifyNone(t)
	client, server := newInProcessTransportPair("localhost", 1)
	c := newChannel(client, 1)
	defer silentClose(c)
	c.setState(SessionStateEstablished)
	reqCmd := createGetPingCommand()
	respCmd := createResponseCommand()
	respCmd.ID = "other-id"
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
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
	ctx, cancel = context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	// Act
	select {
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	case actualRespCmd, ok := <-c.RespCmdChan():
		assert.True(t, ok)
		assert.Equal(t, respCmd, actualRespCmd)
	}
}
