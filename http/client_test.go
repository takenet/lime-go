package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/takenet/lime-go"
)

func testIdentity() lime.Identity {
	return lime.Identity{Name: "bot", Domain: "msging.net"}
}

func TestClientSendMessage_WhenIrisAccepts_ReturnsNoError(t *testing.T) {
	// Arrange
	var receivedPath string
	var receivedAuth string
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "secret-key", WithBaseURL(server.URL))
	msg := &lime.Message{Envelope: lime.Envelope{To: lime.ParseNode("dest@msging.net")}}
	msg.SetContent(lime.TextDocument("hello"))

	// Act
	err := client.SendMessage(context.Background(), msg)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/messages", receivedPath)
	assert.Equal(t, "application/vnd.lime.message+json", receivedContentType)

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net:secret-key"))
	assert.Equal(t, expectedAuth, receivedAuth)
}

func TestClientSendMessage_WhenMessageIsNil_ReturnsError(t *testing.T) {
	client := NewClient(testIdentity(), "secret-key")

	err := client.SendMessage(context.Background(), nil)

	assert.Error(t, err)
}

func TestClientSendNotification_WhenIrisAccepts_ReturnsNoError(t *testing.T) {
	// Arrange
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "secret-key", WithBaseURL(server.URL))
	not := (&lime.Notification{}).SetEvent(lime.NotificationEventReceived)

	// Act
	err := client.SendNotification(context.Background(), not)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/notifications", receivedPath)
}

func TestClientProcessCommand_WhenIrisRespondsSuccess_ReturnsResponseCommand(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/commands", r.URL.Path)
		resp := &lime.ResponseCommand{
			Command: lime.Command{Method: lime.CommandMethodGet},
			Status:  lime.CommandStatusSuccess,
		}
		data, err := json.Marshal(resp)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/vnd.lime.command+json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "secret-key", WithBaseURL(server.URL))
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetURIString("/account")

	// Act
	resp, err := client.ProcessCommand(context.Background(), cmd)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, lime.CommandStatusSuccess, resp.Status)
}

func TestClientProcessCommand_WhenIrisRespondsFailure_ReturnsError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := &lime.ResponseCommand{
			Command: lime.Command{Method: lime.CommandMethodGet},
			Status:  lime.CommandStatusFailure,
			Reason:  &lime.Reason{Code: 404, Description: "not found"},
		}
		data, err := json.Marshal(resp)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "secret-key", WithBaseURL(server.URL))
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetURIString("/account")

	// Act
	resp, err := client.ProcessCommand(context.Background(), cmd)

	// Assert
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, lime.CommandStatusFailure, resp.Status)
}

func TestClientPost_WhenServerReturnsUnauthorized_ReturnsError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "wrong-key", WithBaseURL(server.URL))
	msg := (&lime.Message{}).SetContent(lime.TextDocument("hello"))

	// Act
	err := client.SendMessage(context.Background(), msg)

	// Assert
	assert.Error(t, err)
}

func TestClientPost_WhenContextCanceled_ReturnsError(t *testing.T) {
	// Arrange
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := NewClient(testIdentity(), "secret-key", WithBaseURL(server.URL))
	msg := (&lime.Message{}).SetContent(lime.TextDocument("hello"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Act
	err := client.SendMessage(ctx, msg)

	// Assert
	assert.Error(t, err)
}
