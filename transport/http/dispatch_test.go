package httptransport

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/codec"
	"github.com/takenet/lime-go/protocol"
)

func TestCallbackSynchronousDispatchAndResponseQueue(t *testing.T) {
	var transport *ClientTransport
	var calls int
	settings := Config{BaseURL: "https://iris.example", Identity: protocol.ParseIdentity("app@example")}
	settings.Dispatch = func(ctx context.Context, envelope protocol.EnvelopeValue) error {
		calls++
		if command, ok := envelope.(*protocol.RequestCommand); ok {
			return transport.Send(ctx, command.SuccessResponse())
		}
		return nil
	}
	var err error
	transport, err = NewClientTransport(settings)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, transport.Close()) })
	uri, err := protocol.ParseLimeURI("/ping")
	require.NoError(t, err)
	for _, test := range []struct {
		path     string
		envelope protocol.EnvelopeValue
		status   int
	}{
		{"/commands", &protocol.RequestCommand{Command: protocol.Command{Envelope: protocol.Envelope{ID: "command"}, Method: protocol.CommandMethodGet}, URI: uri}, http.StatusOK},
		{"/messages", (&protocol.Message{}).SetContent(new(protocol.TextDocument)), http.StatusAccepted},
		{"/notifications", &protocol.Notification{Event: protocol.NotificationEventConsumed}, http.StatusAccepted},
	} {
		encoded, err := codec.MarshalEnvelope(test.envelope)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		transport.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, test.path, bytes.NewReader(encoded)))
		require.Equal(t, test.status, recorder.Code, recorder.Body.String())
	}
	require.Equal(t, 3, calls)
	response := &protocol.ResponseCommand{Command: protocol.Command{Envelope: protocol.Envelope{ID: "outbound"}, Method: protocol.CommandMethodGet}, Status: protocol.CommandStatusSuccess}
	encoded, err := codec.MarshalEnvelope(response)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	transport.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(encoded)))
	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.Equal(t, 3, calls, "response callbacks must not enter business dispatch")
	queued, err := transport.Receive(context.Background())
	require.NoError(t, err)
	require.Equal(t, response, queued)
}

func TestSynchronousCommandDispatchTimeoutAndError(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		settings := Config{BaseURL: "https://iris.example", Identity: protocol.ParseIdentity("app@example"), CallbackCommandTimeout: time.Millisecond}
		settings.Dispatch = func(ctx context.Context, _ protocol.EnvelopeValue) error {
			if deadline {
				<-ctx.Done()
				return ctx.Err()
			}
			return errors.New("dispatch failed")
		}
		transport, err := NewClientTransport(settings)
		require.NoError(t, err)
		uri, err := protocol.ParseLimeURI("/ping")
		require.NoError(t, err)
		command := &protocol.RequestCommand{Command: protocol.Command{Envelope: protocol.Envelope{ID: "request"}, Method: protocol.CommandMethodGet}, URI: uri}
		encoded, err := codec.MarshalEnvelope(command)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		transport.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(encoded)))
		status := http.StatusServiceUnavailable
		if deadline {
			status = http.StatusGatewayTimeout
		}
		require.Equal(t, status, recorder.Code)
		require.Empty(t, transport.pending)
		require.NoError(t, transport.Close())
	}
}

func TestSynchronousDispatchReservesIDBeforeHandler(t *testing.T) {
	var transport *ClientTransport
	uri, err := protocol.ParseLimeURI("/ping")
	require.NoError(t, err)
	command := &protocol.RequestCommand{Command: protocol.Command{Envelope: protocol.Envelope{ID: "duplicate"}, Method: protocol.CommandMethodGet}, URI: uri}
	encoded, err := codec.MarshalEnvelope(command)
	require.NoError(t, err)
	settings := Config{BaseURL: "https://iris.example", Identity: protocol.ParseIdentity("app@example")}
	settings.Dispatch = func(ctx context.Context, envelope protocol.EnvelopeValue) error {
		nested := httptest.NewRecorder()
		transport.ServeHTTP(nested, httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(encoded)))
		require.Equal(t, http.StatusConflict, nested.Code)
		return transport.Send(ctx, envelope.(*protocol.RequestCommand).SuccessResponse())
	}
	transport, err = NewClientTransport(settings)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, transport.Close()) })
	recorder := httptest.NewRecorder()
	transport.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(encoded)))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, transport.pending)
}

func TestSynchronousDispatchCancellationAndTransportClose(t *testing.T) {
	for _, closeTransport := range []bool{false, true} {
		entered := make(chan struct{})
		settings := Config{BaseURL: "https://iris.example", Identity: protocol.ParseIdentity("app@example")}
		settings.Dispatch = func(ctx context.Context, _ protocol.EnvelopeValue) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		}
		transport, err := NewClientTransport(settings)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, transport.Close()) })
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			<-entered
			if closeTransport {
				require.NoError(t, transport.Close())
			} else {
				cancel()
			}
		}()
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/commands", bytes.NewBufferString(`{"id":"request","method":"get","uri":"/ping"}`)).WithContext(ctx)
		transport.ServeHTTP(recorder, request)
		expected := http.StatusRequestTimeout
		if closeTransport {
			expected = http.StatusServiceUnavailable
		}
		require.Equal(t, expected, recorder.Code)
		<-stopped
		require.Empty(t, transport.pending)
	}
}

func TestCommandEmptyAcknowledgmentRequiresAsyncCallbacks(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   int
		callback bool
		session  bool
		success  bool
	}{
		{"accepted command with callbacks", http.StatusAccepted, true, false, true},
		{"accepted command without callbacks", http.StatusAccepted, false, false, false},
		{"empty synchronous command", http.StatusOK, true, false, false},
		{"empty session", http.StatusAccepted, true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method == http.MethodGet {
					writer.WriteHeader(http.StatusNotFound)
					return
				}
				writer.WriteHeader(test.status)
			}))
			defer server.Close()
			settings := Config{BaseURL: server.URL, Identity: protocol.ParseIdentity("app@example")}
			if test.callback {
				settings.ReceiveBaseUri = "http://application.test"
			}
			transport, err := NewClientTransport(settings)
			require.NoError(t, err)
			defer transport.Close()
			uri, err := protocol.ParseLimeURI("/ping")
			require.NoError(t, err)
			command := &protocol.RequestCommand{Command: protocol.Command{
				Envelope: protocol.Envelope{ID: "outbound"}, Method: protocol.CommandMethodGet,
			}, URI: uri}
			var envelope protocol.EnvelopeValue = command
			if test.session {
				envelope = &protocol.Session{State: protocol.SessionStateNew}
			}
			err = transport.Send(context.Background(), envelope)
			if !test.success {
				require.ErrorContains(t, err, "empty response envelope")
				return
			}
			require.NoError(t, err)
			encoded, err := codec.MarshalEnvelope(command.SuccessResponse())
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			transport.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/commands", bytes.NewReader(encoded)))
			require.Equal(t, http.StatusAccepted, recorder.Code)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			response, err := transport.Receive(ctx)
			require.NoError(t, err)
			require.Equal(t, command.SuccessResponse(), response)
		})
	}
}
