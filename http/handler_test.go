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

	lime "github.com/takenet/lime-go/protocol"
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

func TestHandlerServeHTTP_WhenCommandHandlerFails_ReturnsSanitizedLimeFailure(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		CommandHandler: func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
			return nil, errors.New("sensitive internal detail")
		},
	}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetID("command-id")
	cmd.SetURIString("/resource")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response lime.ResponseCommand
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, lime.CommandStatusFailure, response.Status)
	require.NotNil(t, response.Reason)
	assert.Equal(t, 61, response.Reason.Code)
	assert.NotContains(t, rec.Body.String(), "sensitive internal detail")
}

func TestHandlerServeHTTP_WhenCommandHandlerReturnsTypedFailure_PreservesReason(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		CommandHandler: func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
			return nil, NewCommandError(lime.Reason{Code: 64, Description: "Invalid schedule"}, errors.New("invalid date"))
		},
	}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodSet}}
	cmd.SetID("command-id")
	cmd.SetURIString("/schedules")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response lime.ResponseCommand
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.NotNil(t, response.Reason)
	assert.Equal(t, 64, response.Reason.Code)
	assert.Equal(t, "Invalid schedule", response.Reason.Description)
	assert.NotContains(t, rec.Body.String(), "invalid date")
}

func TestHandlerServeHTTP_WhenMuxHasNoMatchingCommand_ReturnsLimeFailure(t *testing.T) {
	mux := &CommandMux{}
	h := &Handler{
		Authenticator:  allowAllAuthenticator(),
		CommandHandler: mux.Handle,
	}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodGet}}
	cmd.SetID("command-id")
	cmd.SetURIString("/unknown")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var response lime.ResponseCommand
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, lime.CommandStatusFailure, response.Status)
	require.NotNil(t, response.Reason)
	assert.Equal(t, lime.ReasonCodeCommandResourceNotSupported, response.Reason.Code)
}

func TestHandlerServeHTTP_WhenObserveWithoutIDReturnsNoResponse_ReturnsAccepted(t *testing.T) {
	h := &Handler{
		Authenticator: allowAllAuthenticator(),
		CommandHandler: func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
			return nil, nil
		},
	}
	cmd := &lime.RequestCommand{Command: lime.Command{Method: lime.CommandMethodObserve}}
	cmd.SetURIString("/presence")
	data, err := json.Marshal(cmd)
	require.NoError(t, err)

	req := newSignedRequest(t, string(data), "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Body.String())
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
