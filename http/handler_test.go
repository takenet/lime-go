package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/takenet/lime-go"
)

func newSignedRequest(t *testing.T, body string, contentType string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("bot@msging.net:secret-key"))
	req.Header.Set("Authorization", auth)
	return req
}

func allowAllAuthenticator() AuthenticatorFunc {
	return func(ctx context.Context, identity lime.Identity, scheme lime.AuthenticationScheme, credential string) (bool, error) {
		return credential == "secret-key", nil
	}
}

func TestHandlerServeHTTP_WhenMethodNotPost_ReturnsMethodNotAllowed(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestHandlerServeHTTP_WhenContentTypeUnsupported_ReturnsUnsupportedMediaType(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
}

func TestHandlerServeHTTP_WhenAuthorizationHeaderMissing_ReturnsUnauthorized(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandlerServeHTTP_WhenCredentialsInvalid_ReturnsUnauthorized(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	req := newSignedRequest(t, "{}", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("bot@msging.net:wrong-key")))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandlerServeHTTP_WhenBodyMalformed_ReturnsBadRequest(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	req := newSignedRequest(t, "not json", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandlerServeHTTP_WhenMessageReceivedWithoutHandler_ReturnsAccepted(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	msg := (&lime.Message{}).SetContent(lime.TextDocument("hi"))
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestHandlerServeHTTP_WhenMessageHandlerReturnsNotification_WritesNotification(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		MessageHandler: func(ctx context.Context, msg *lime.Message) (*lime.Notification, error) {
			return msg.Notification(lime.NotificationEventConsumed), nil
		},
	}
	msg := (&lime.Message{}).SetContent(lime.TextDocument("hi"))
	msg.ID = "1"
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var not lime.Notification
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &not))
	assert.Equal(t, lime.NotificationEventConsumed, not.Event)
}

func TestHandlerServeHTTP_WhenMessageHandlerFails_ReturnsInternalServerError(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		MessageHandler: func(ctx context.Context, msg *lime.Message) (*lime.Notification, error) {
			return nil, errors.New("boom")
		},
	}
	msg := (&lime.Message{}).SetContent(lime.TextDocument("hi"))
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandlerServeHTTP_WhenNotificationReceived_ReturnsAccepted(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	not := (&lime.Notification{}).SetEvent(lime.NotificationEventReceived)
	data, err := json.Marshal(not)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestHandlerServeHTTP_WhenCommandReceivedWithoutHandler_ReturnsNotImplemented(t *testing.T) {
	h := &Handler{Authenticator: allowAllAuthenticator()}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetURIString("/account")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestHandlerServeHTTP_WhenCommandHandled_ReturnsResponseCommand(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		CommandHandler: func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
			return cmd.SuccessResponse(), nil
		},
	}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetURIString("/account")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp lime.ResponseCommand
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, lime.CommandStatusSuccess, resp.Status)
}

func TestHandlerServeHTTP_WhenNoAuthenticatorConfigured_SkipsCredentialCheck(t *testing.T) {
	h := &Handler{}
	not := (&lime.Notification{}).SetEvent(lime.NotificationEventReceived)
	data, err := json.Marshal(not)
	require.NoError(t, err)

	// Still requires a well-formed Authorization header, but any credential
	// value is accepted since no Authenticator is configured.
	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
}
