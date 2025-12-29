package lime

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func TestNewStdoutTraceWriter(t *testing.T) {
	// Act
	writer := NewStdoutTraceWriter()
	defer func() { _ = writer.Close() }()

	// Assert
	assert.NotNil(t, writer)
	assert.NotNil(t, writer.SendWriter())
	assert.NotNil(t, writer.ReceiveWriter())
}

func TestTraceWriterSendWriter(t *testing.T) {
	// Arrange
	sendBuf := &bytes.Buffer{}
	receiveBuf := &bytes.Buffer{}
	writer := &StdoutTraceWriter{
		sendWriter:    nopWriteCloser{sendBuf},
		receiveWriter: nopWriteCloser{receiveBuf},
	}

	// Act
	send := writer.SendWriter()

	// Assert
	assert.NotNil(t, send)
}

func TestTraceWriterReceiveWriter(t *testing.T) {
	// Arrange
	sendBuf := &bytes.Buffer{}
	receiveBuf := &bytes.Buffer{}
	writer := &StdoutTraceWriter{
		sendWriter:    nopWriteCloser{sendBuf},
		receiveWriter: nopWriteCloser{receiveBuf},
	}

	// Act
	receive := writer.ReceiveWriter()

	// Assert
	assert.NotNil(t, receive)
}

func TestTraceWriterWithCustomWriters(t *testing.T) {
	// Arrange
	sendBuf := &bytes.Buffer{}
	receiveBuf := &bytes.Buffer{}

	// Act
	writer := &StdoutTraceWriter{
		sendWriter:    nopWriteCloser{sendBuf},
		receiveWriter: nopWriteCloser{receiveBuf},
	}

	// Write to send
	_, err := (*writer.SendWriter()).Write([]byte("test send"))
	assert.NoError(t, err)

	// Write to receive
	_, err = (*writer.ReceiveWriter()).Write([]byte("test receive"))
	assert.NoError(t, err)

	// Assert
	assert.Equal(t, "test send", sendBuf.String())
	assert.Equal(t, "test receive", receiveBuf.String())
}

func TestNewTraceWriter(t *testing.T) {
	// Arrange
	sendBuf := &bytes.Buffer{}
	receiveBuf := &bytes.Buffer{}

	// Act
	writer := &StdoutTraceWriter{
		sendWriter:    nopWriteCloser{sendBuf},
		receiveWriter: nopWriteCloser{receiveBuf},
	}

	// Assert
	assert.NotNil(t, writer)
	assert.NotNil(t, writer.SendWriter())
	assert.NotNil(t, writer.ReceiveWriter())
}

type discardWriter struct{}

func (d discardWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func TestTraceWriterWithDiscardWriter(t *testing.T) {
	// Arrange
	discard := discardWriter{}
	writer := &StdoutTraceWriter{
		sendWriter:    nopWriteCloser{discard},
		receiveWriter: nopWriteCloser{discard},
	}

	// Act & Assert
	n, err := (*writer.SendWriter()).Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)

	n, err = (*writer.ReceiveWriter()).Write([]byte("test"))
	assert.NoError(t, err)
	assert.Equal(t, 4, n)
}

func TestTraceWriterImplementsInterface(t *testing.T) {
	// Arrange
	var _ TraceWriter = (*StdoutTraceWriter)(nil)
	var _ io.Writer = discardWriter{}
}
