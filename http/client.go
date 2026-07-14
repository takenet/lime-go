package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/takenet/lime-go"
)

// defaultBaseURL is Blip's public HTTP gateway endpoint.
const defaultBaseURL = "https://http.msging.net"

// defaultTimeout is the default per-request timeout used by Client when none
// is configured.
const defaultTimeout = 30 * time.Second

// Client sends LIME envelopes to Iris (Blip's HTTP gateway) using the
// stateless LIME-over-HTTP protocol: every request is authenticated
// individually via an "Authorization: Basic base64(identity:key)" header and
// carries a single envelope as its JSON body.
type Client struct {
	// BaseURL is the Iris endpoint base address, e.g. "https://http.msging.net".
	BaseURL string
	// Identity is the identity used to authenticate every request.
	Identity lime.Identity
	// Key is the access key (or password) used to authenticate every request.
	Key string
	// HTTPClient is the underlying HTTP client used to perform requests.
	HTTPClient *http.Client
	// Timeout bounds each request when the provided context has no deadline.
	Timeout time.Duration
}

// ClientOption configures optional Client parameters.
type ClientOption func(*Client)

// WithBaseURL overrides the default Iris base URL.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.BaseURL = baseURL
	}
}

// WithHTTPClient overrides the underlying *http.Client used to perform requests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.HTTPClient = httpClient
	}
}

// WithTimeout overrides the default per-request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// NewClient creates a Client that authenticates as identity using key,
// targeting Iris' public endpoint by default.
func NewClient(identity lime.Identity, key string, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:    defaultBaseURL,
		Identity:   identity,
		Key:        key,
		HTTPClient: http.DefaultClient,
		Timeout:    defaultTimeout,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SendMessage sends a message to Iris. Iris responds with an empty "202
// Accepted" for successfully accepted messages.
func (c *Client) SendMessage(ctx context.Context, msg *lime.Message) error {
	if msg == nil {
		return fmt.Errorf("send message: message cannot be nil")
	}

	_, err := c.post(ctx, "/messages", msg)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	return nil
}

// SendNotification sends a notification to Iris. Iris responds with an empty
// "202 Accepted" for successfully accepted notifications.
func (c *Client) SendNotification(ctx context.Context, not *lime.Notification) error {
	if not == nil {
		return fmt.Errorf("send notification: notification cannot be nil")
	}

	_, err := c.post(ctx, "/notifications", not)
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	return nil
}

// ProcessCommand sends a request command to Iris and returns the synchronous
// response command. An error is returned if the transport request fails or
// if the response command's status is CommandStatusFailure, in which case
// the error wraps the response's Reason.
func (c *Client) ProcessCommand(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
	if cmd == nil {
		return nil, fmt.Errorf("process command: command cannot be nil")
	}

	body, err := c.post(ctx, "/commands", cmd)
	if err != nil {
		return nil, fmt.Errorf("process command: %w", err)
	}

	respCmd := &lime.ResponseCommand{}
	if err := json.Unmarshal(body, respCmd); err != nil {
		return nil, fmt.Errorf("process command: decode response: %w", err)
	}

	if respCmd.Status == lime.CommandStatusFailure {
		reason := "unknown reason"
		if respCmd.Reason != nil {
			reason = respCmd.Reason.String()
		}
		return respCmd, fmt.Errorf("process command: command failed: %s", reason)
	}

	return respCmd, nil
}

// post marshals env, sends it as a POST request to path and returns the raw
// response body when the request succeeds (2xx status).
func (c *Client) post(ctx context.Context, path string, env any) ([]byte, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}

	contentType, err := envelopeContentType(env)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("marshal envelope: %w", err)
	}

	if c.Timeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.Timeout)
			defer cancel()
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", buildBasicAuthorizationHeader(c.Identity, c.Key))

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized (status %d)", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
