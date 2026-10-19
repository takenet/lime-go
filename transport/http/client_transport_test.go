package httptransport

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/codec"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/tracing"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestClientTransportCallbackTraceContext(t *testing.T) {
	const httpParent = "00-11111111111111111111111111111111-1111111111111111-01"
	const envelopeParent = "00-22222222222222222222222222222222-2222222222222222-01"
	const activeParent = "00-33333333333333333333333333333333-3333333333333333-01"
	for _, test := range []struct {
		name, metadata, header, active, want string
	}{
		{name: "HTTP header", header: httpParent, want: httpParent},
		{name: "envelope wins", metadata: envelopeParent, header: httpParent, want: envelopeParent},
		{name: "invalid envelope fallback", metadata: "invalid", header: httpParent, want: httpParent},
		{name: "active HTTP span", header: httpParent, active: activeParent, want: activeParent},
		{name: "envelope beats active span", metadata: envelopeParent, active: activeParent, want: envelopeParent},
		{name: "invalid HTTP parent", header: "invalid"},
		{name: "no parent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClientTransport(Config{BaseURL: "https://iris.example.com",
				Identity: protocol.Identity{Name: "bot", Domain: "example.com"}, ReceiveBaseUri: "https://bot.example.com/lime"})
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			message := &protocol.Message{Envelope: protocol.Envelope{Metadata: map[string]string{tracing.TraceParent: test.metadata}}}
			content := protocol.TextDocument("hello")
			message.SetContent(&content)
			data, err := codec.MarshalEnvelope(message)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodPost, "/messages", bytes.NewReader(data))
			request.Header.Set(tracing.TraceParent, test.header)
			if test.active != "" {
				ctx := propagation.TraceContext{}.Extract(request.Context(), propagation.MapCarrier{tracing.TraceParent: test.active})
				request = request.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			client.ServeHTTP(response, request)
			require.Equal(t, http.StatusAccepted, response.Code)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			received, err := client.Receive(ctx)
			require.NoError(t, err)
			actual := trace.SpanContextFromContext(tracing.ExtractContext(ctx, received))
			expected := propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{tracing.TraceParent: test.want})
			require.Equal(t, trace.SpanContextFromContext(expected), actual)
		})
	}
}

func TestClientTransportPostsSessionAndUsesSessionHeaders(t *testing.T) {
	const parent = "00-11111111111111111111111111111111-1111111111111111-01"
	requests := make(chan *http.Request, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, parent, request.Header.Get(tracing.TraceParent))
		if request.Method == http.MethodGet && request.URL.Path == "/sessions" {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		requests <- request.Clone(context.Background())
		if request.URL.Path == "/sessions" {
			session := &protocol.Session{
				Envelope: protocol.Envelope{ID: "lime-session"},
				State:    protocol.SessionStateEstablished,
			}
			data, err := codec.MarshalEnvelope(session)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
			writer.Header().Set(HeaderSessionID, "http-session")
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write(data)
			return
		}
		writer.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	identity := protocol.Identity{Name: "bot", Domain: "example.com"}
	client, err := NewClientTransport(Config{
		BaseURL:             server.URL,
		Identity:            identity,
		ReceiveBaseUri:      "https://bot.example.com/lime",
		SendSessionIDHeader: true,
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := propagation.TraceContext{}.Extract(context.Background(), propagation.MapCarrier{tracing.TraceParent: parent})
	require.NoError(t, client.Send(ctx, &protocol.Session{State: protocol.SessionStateNew}))
	envelope, err := client.Receive(context.Background())
	require.NoError(t, err)
	session, ok := envelope.(*protocol.Session)
	require.True(t, ok)
	assert.Equal(t, protocol.SessionStateEstablished, session.State)

	message := &protocol.Message{}
	content := protocol.TextDocument("hello")
	message.SetContent(&content)
	require.NoError(t, client.Send(ctx, message))

	sessionRequest := <-requests
	assert.Equal(t, "/sessions", sessionRequest.URL.Path)
	assert.Equal(t, identity.String(), sessionRequest.Header.Get(HeaderSessionIdentity))
	assert.Equal(t, "-1", sessionRequest.Header.Get(HeaderSessionExpiration))
	assert.Equal(t, "identity", sessionRequest.Header.Get(HeaderSessionPresenceRoutingRule))
	assert.Equal(t, "https://bot.example.com/lime/messages", sessionRequest.Header.Get(HeaderSessionMessageURL))
	assert.Equal(t, "https://bot.example.com/lime/notifications", sessionRequest.Header.Get(HeaderSessionNotificationURL))
	assert.Equal(t, "https://bot.example.com/lime/commands", sessionRequest.Header.Get(HeaderSessionCommandURL))

	messageRequest := <-requests
	assert.Equal(t, "/messages", messageRequest.URL.Path)
	assert.Equal(t, "http-session", messageRequest.Header.Get(HeaderSessionID))
}

func TestClientTransportCallbackCommandReturnsCorrelatedResponse(t *testing.T) {
	callbackMux := NewCallbackMux()
	client, err := NewClientTransport(Config{
		BaseURL:        "https://iris.example.com",
		Identity:       protocol.Identity{Name: "bot", Domain: "example.com"},
		ReceiveBaseUri: "https://bot.example.com/lime",
		CallbackMux:    callbackMux,
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	client.storeSessionID("http-session")

	command := &protocol.RequestCommand{Command: protocol.Command{
		Envelope: protocol.Envelope{ID: "command-1"},
		Method:   protocol.CommandMethodGet,
	}}
	command.SetURIString("/ping")
	data, err := codec.MarshalEnvelope(command)
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, client.callbackPath+"/commands", bytes.NewReader(data))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(HeaderSessionID, "http-session")
	request.Header.Set(HeaderCorrelationID, "correlation-1")
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		callbackMux.ServeHTTP(response, request)
		close(done)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	envelope, err := client.Receive(ctx)
	require.NoError(t, err)
	received, ok := envelope.(*protocol.RequestCommand)
	require.True(t, ok)
	assert.Equal(t, command.ID, received.ID)
	require.NoError(t, client.Send(ctx, received.SuccessResponse()))
	<-done

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "http-session", response.Header().Get(HeaderSessionID))
	assert.Equal(t, "correlation-1", response.Header().Get(HeaderCorrelationID))
	assert.Equal(t, "bot@example.com", response.Header().Get(HeaderSessionIdentity))
	result, err := codec.UnmarshalEnvelope(response.Body.Bytes())
	require.NoError(t, err)
	responseCommand, ok := result.(*protocol.ResponseCommand)
	require.True(t, ok)
	assert.Equal(t, command.ID, responseCommand.ID)
	assert.Equal(t, protocol.CommandStatusSuccess, responseCommand.Status)

	require.NoError(t, client.Close())
	closedCallback := httptest.NewRecorder()
	closedRequest := httptest.NewRequest(http.MethodPost, client.callbackPath+"/messages", nil)
	closedRequest.Header.Set(HeaderSessionID, "http-session")
	callbackMux.ServeHTTP(closedCallback, closedRequest)
	assert.Equal(t, http.StatusNotFound, closedCallback.Code)
}

func TestClientTransportPollsForIncomingEnvelope(t *testing.T) {
	const parent = "00-11111111111111111111111111111111-1111111111111111-01"
	var delivered atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/messages" && delivered.CompareAndSwap(false, true) {
			writer.Header().Set(tracing.TraceParent, parent)
			message := &protocol.Message{}
			content := protocol.TextDocument("from Iris")
			message.SetContent(&content)
			data, err := codec.MarshalEnvelope(message)
			if err != nil {
				http.Error(writer, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = writer.Write(data)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClientTransport(Config{
		BaseURL:      server.URL,
		Identity:     protocol.Identity{Name: "bot", Domain: "example.com"},
		PollTimeout:  time.Second,
		PollInterval: time.Millisecond,
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	envelope, err := client.Receive(ctx)
	require.NoError(t, err)
	message, ok := envelope.(*protocol.Message)
	require.True(t, ok)
	require.Equal(t, parent, message.Metadata[tracing.TraceParent])
	raw, ok := message.Content.(*protocol.TextDocument)
	require.True(t, ok)
	assert.Equal(t, protocol.TextDocument("from Iris"), *raw)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (roundTrip roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestClientTransportPollingRetainsResponsesAfterReceiveCancellation(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var requests sync.Map
	client, err := NewClientTransport(Config{
		BaseURL:  "https://iris.example.com",
		Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if _, loaded := requests.LoadOrStore(request.URL.Path, true); loaded {
				<-request.Context().Done()
				return nil, request.Context().Err()
			}
			started <- request.URL.Path
			<-release
			bodies := map[string]string{
				"/messages":      `{"id":"message","type":"text/plain","content":"hello"}`,
				"/notifications": `{"id":"notification","event":"received"}`,
				"/commands":      `{"id":"command","method":"get","uri":"/ping"}`,
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(bodies[request.URL.Path])), Header: make(http.Header)}, nil
		})},
	})
	require.NoError(t, err)
	defer func() { releaseOnce.Do(func() { close(release) }); _ = client.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	received := make(chan error, 1)
	go func() { _, receiveErr := client.Receive(ctx); received <- receiveErr }()
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("pollers did not start")
		}
	}
	cancel()
	require.ErrorIs(t, <-received, context.Canceled)
	releaseOnce.Do(func() { close(release) })

	nextCtx, nextCancel := context.WithTimeout(context.Background(), time.Second)
	defer nextCancel()
	ids := make(map[string]bool)
	for range 3 {
		envelope, receiveErr := client.Receive(nextCtx)
		require.NoError(t, receiveErr)
		switch value := envelope.(type) {
		case *protocol.Message:
			ids[value.ID] = true
		case *protocol.Notification:
			ids[value.ID] = true
		case *protocol.RequestCommand:
			ids[value.ID] = true
		default:
			t.Fatalf("unexpected envelope %T", envelope)
		}
	}
	assert.Equal(t, map[string]bool{"message": true, "notification": true, "command": true}, ids)
}

func TestClientTransportSessionPreflight(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		mutate    func(http.Header)
		wantReset bool
		wantError bool
		polling   bool
	}{
		{name: "matching established session"},
		{name: "missing session", status: http.StatusNotFound},
		{name: "old callback", mutate: func(headers http.Header) {
			headers.Set(HeaderSessionMessageURL, "https://bot.example.com/old/messages")
		}, wantReset: true},
		{name: "callback path case matters", mutate: func(headers http.Header) {
			headers.Set(HeaderSessionCommandURL, "https://bot.example.com/LIME/commands")
		}, wantReset: true},
		{name: "callback credentials case matters", mutate: func(headers http.Header) {
			headers.Set(HeaderSessionCommandHeaders, strings.Replace(headers.Get(HeaderSessionCommandHeaders), "callback", "CALLBACK", 1))
		}, wantReset: true},
		{name: "polling removes old callback", polling: true, mutate: func(headers http.Header) {
			headers.Set(HeaderSessionMessageURL, "https://bot.example.com/old/messages")
		}, wantReset: true},
		{name: "changed presence", mutate: func(headers http.Header) { headers.Set(HeaderSessionPresenceRoutingRule, "instance") }, wantReset: true},
		{name: "changed receipts", mutate: func(headers http.Header) { headers.Set(HeaderSessionReceiptEvents, "failed") }, wantReset: true},
		{name: "missing config", mutate: func(headers http.Header) { headers.Del(HeaderSessionExpiration) }, wantReset: true},
		{name: "csharp enum casing", mutate: func(headers http.Header) {
			headers.Set(HeaderSessionPresenceRoutingRule, "Identity")
			headers.Set(HeaderSessionPresencePromiscuous, "False")
			headers[http.CanonicalHeaderKey(HeaderSessionReceiptEvents)] = []string{"Accepted", "Received", "Consumed", "Failed"}
		}},
		{name: "unestablished session", body: `{"id":"old","state":"failed"}`, wantReset: true},
		{name: "unauthorized", status: http.StatusUnauthorized, wantError: true},
		{name: "server failure", status: http.StatusInternalServerError, wantError: true},
		{name: "invalid JSON", body: `invalid`, wantError: true},
		{name: "wrong envelope", body: `{"id":"wrong","event":"received"}`, wantError: true},
		{name: "oversized body", body: strings.Repeat("x", 513), wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var methods []string
			callbackURL := "https://bot.example.com/lime"
			if test.polling {
				callbackURL = ""
			}
			client, err := NewClientTransport(Config{
				BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
				ReceiveBaseUri: callbackURL, MaxBodyBytes: 512,
				Headers:         http.Header{"Authorization": {"Bearer outbound"}, "X-Custom": {"forwarded"}},
				CallbackHeaders: http.Header{"Authorization": {"Bearer callback"}},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					methods = append(methods, request.Method)
					assert.Equal(t, "/sessions", request.URL.Path)
					assert.Equal(t, "Bearer outbound", request.Header.Get("Authorization"))
					assert.Equal(t, "forwarded", request.Header.Get("X-Custom"))
					assert.Empty(t, request.Header.Get(HeaderSessionID))
					headers := request.Header.Clone()
					status := http.StatusOK
					body := `{"id":"existing","state":"established"}`
					if request.Method == http.MethodGet {
						assert.Empty(t, request.Header.Get(HeaderSession))
						if test.mutate != nil {
							test.mutate(headers)
						}
						if test.status != 0 {
							status = test.status
						}
						if test.body != "" {
							body = test.body
						}
					} else {
						assert.Equal(t, test.wantReset, request.Header.Get(HeaderSession) == "reset")
						headers.Set(HeaderSessionID, "new-http-session")
					}
					return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
				})},
			})
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			err = client.Send(context.Background(), &protocol.Session{State: protocol.SessionStateNew})
			if test.wantError {
				require.Error(t, err)
				assert.Equal(t, []string{http.MethodGet}, methods)
				assert.Empty(t, client.sessionID())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{http.MethodGet, http.MethodPost}, methods)
			assert.Equal(t, "new-http-session", client.sessionID())
		})
	}
}

func TestClientTransportPresenceDefaultsAndHeaderIsolation(t *testing.T) {
	for _, test := range []struct{ name, callback, rule, want string }{
		{"polling default", "", "", "instance"},
		{"callback default", "https://bot.example.com/lime", "", "identity"},
		{"explicit callback", "https://bot.example.com/lime", "domain", "domain"},
		{"explicit polling", "", "identity", "identity"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outbound := http.Header{"Authorization": {"Bearer outbound"}}
			callbacks := http.Header{"Authorization": {"Bearer callback=="}}
			client, err := NewClientTransport(Config{
				BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
				ReceiveBaseUri: test.callback, PresenceRoutingRule: test.rule, Headers: outbound, CallbackHeaders: callbacks,
			})
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			outbound.Set("Authorization", "mutated")
			callbacks.Set("Authorization", "mutated")
			headers := client.requestHeaders(true)
			assert.Equal(t, test.want, headers.Get(HeaderSessionPresenceRoutingRule))
			assert.Equal(t, "Bearer outbound", headers.Get("Authorization"))
			if test.callback == "" {
				assert.Empty(t, headers.Get(HeaderSessionMessageHeaders))
				return
			}
			forwarded := headers.Get(HeaderSessionMessageHeaders)
			assert.Contains(t, forwarded, "Authorization=Bearer callback==")
			assert.Contains(t, forwarded, "Http-Session-Identity=bot@example.com")
			assert.Contains(t, strings.ToLower(forwarded), "server-should-store=true")
			assert.NotContains(t, forwarded, "outbound")
		})
	}
}

func TestClientTransportRejectsAmbiguousCallbackHeaders(t *testing.T) {
	for _, headers := range []http.Header{
		{"Authorization": {"Bearer secret;Injected=value"}},
		{"Authorization": {"Bearer secret\r\nInjected: value"}},
		{"Authorization": {"one", "two"}},
		{"Bad=Name": {"value"}},
		{"": {"value"}},
	} {
		client, err := NewClientTransport(Config{
			BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
			ReceiveBaseUri: "https://bot.example.com/lime", CallbackHeaders: headers,
		})
		require.Error(t, err)
		assert.Nil(t, client)
		assert.NotContains(t, err.Error(), "secret")
	}
}

func TestClientTransportCloseCancelsAndJoinsPollers(t *testing.T) {
	started := make(chan struct{}, 3)
	canceled := make(chan struct{}, 3)
	release := make(chan struct{})
	var releaseOnce sync.Once
	client, err := NewClientTransport(Config{
		BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			started <- struct{}{}
			<-request.Context().Done()
			canceled <- struct{}{}
			<-release
			return nil, request.Context().Err()
		})},
	})
	require.NoError(t, err)
	defer func() { releaseOnce.Do(func() { close(release) }); _ = client.Close() }()
	client.startPolling()
	for range 3 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("poller did not start")
		}
	}
	closed := make(chan struct{})
	go func() { _ = client.Close(); close(closed) }()
	for range 3 {
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("Close did not cancel poller")
		}
	}
	select {
	case <-closed:
		t.Fatal("Close returned before pollers exited")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not join pollers")
	}
	assert.False(t, client.Connected())
	_, err = client.Receive(context.Background())
	require.Error(t, err)
	require.NoError(t, client.Close())
}

func TestClientTransportPollingBackpressureIsBoundedAndCloseUnblocks(t *testing.T) {
	var requests atomic.Int64
	client, err := NewClientTransport(Config{
		BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
		PollInterval: time.Nanosecond,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests.Add(1)
			bodies := map[string]string{
				"/messages":      `{"type":"text/plain","content":"hello"}`,
				"/notifications": `{"id":"notification","event":"received"}`,
				"/commands":      `{"id":"command","method":"get","uri":"/ping"}`,
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(bodies[request.URL.Path]))}, nil
		})},
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	client.startPolling()
	require.Eventually(t, func() bool { return requests.Load() == int64(cap(client.incoming)+3) }, time.Second, time.Millisecond)
	assert.Len(t, client.incoming, cap(client.incoming))
	closed := make(chan struct{})
	go func() { _ = client.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close blocked on full inbox")
	}
	assert.Equal(t, int64(cap(client.incoming)+3), requests.Load())
}

func TestClientTransportPollingTimeoutRetriesAndSurfacesErrors(t *testing.T) {
	var attempts atomic.Int64
	client, err := NewClientTransport(Config{
		BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
		PollTimeout: time.Millisecond, PollInterval: time.Millisecond,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/messages" || attempts.Add(1) == 1 {
				<-request.Context().Done()
				return nil, request.Context().Err()
			}
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("denied"))}, nil
		})},
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = client.Receive(ctx)
	require.ErrorContains(t, err, "poll messages returned status 403")
	assert.GreaterOrEqual(t, attempts.Load(), int64(2))
}

func TestClientTransportCallbackBounds(t *testing.T) {
	for _, test := range []struct {
		name, method, path, body, authorization, session string
		want                                             int
	}{
		{name: "valid", method: http.MethodPost, path: "/messages", body: `{"type":"text/plain","content":"hello"}`, authorization: "Bearer callback", session: "http-session", want: http.StatusAccepted},
		{name: "missing auth", method: http.MethodPost, path: "/messages", want: http.StatusUnauthorized},
		{name: "wrong auth", method: http.MethodPost, path: "/messages", authorization: "Bearer outbound", want: http.StatusUnauthorized},
		{name: "wrong session", method: http.MethodPost, path: "/messages", authorization: "Bearer callback", session: "wrong", want: http.StatusUnauthorized},
		{name: "wrong method", method: http.MethodGet, path: "/messages", want: http.StatusMethodNotAllowed},
		{name: "wrong path", method: http.MethodPost, path: "/sessions", authorization: "Bearer callback", want: http.StatusNotFound},
		{name: "malformed", method: http.MethodPost, path: "/messages", body: "invalid", authorization: "Bearer callback", want: http.StatusBadRequest},
		{name: "wrong envelope", method: http.MethodPost, path: "/messages", body: `{"id":"notice","event":"received"}`, authorization: "Bearer callback", want: http.StatusBadRequest},
		{name: "oversized", method: http.MethodPost, path: "/messages", body: strings.Repeat("x", 129), authorization: "Bearer callback", want: http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, err := NewClientTransport(Config{
				BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"}, MaxBodyBytes: 128,
				ReceiveBaseUri:        "https://bot.example.com/lime",
				CallbackAuthenticator: func(request *http.Request) bool { return request.Header.Get("Authorization") == "Bearer callback" },
			})
			require.NoError(t, err)
			defer func() { _ = client.Close() }()
			client.storeSessionID("http-session")
			client.storeLimeSession(&protocol.Session{Envelope: protocol.Envelope{ID: "lime-session", From: protocol.ParseNode("iris@example.com/server"), To: protocol.ParseNode("bot@example.com/client")}, State: protocol.SessionStateEstablished})
			request := httptest.NewRequest(test.method, "/lime"+test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", test.authorization)
			request.Header.Set(HeaderSessionID, test.session)
			request.Header.Set(HeaderCorrelationID, "correlation")
			response := httptest.NewRecorder()
			client.ServeHTTP(response, request)
			assert.Equal(t, test.want, response.Code)
			assert.Empty(t, response.Header().Get("Authorization"))
			if test.want == http.StatusUnauthorized {
				assert.Empty(t, response.Header().Get(HeaderSessionID))
				assert.Empty(t, response.Header().Get(HeaderLimeSessionID))
			}
			if test.want == http.StatusAccepted {
				assert.Len(t, client.incoming, 1)
				assert.Equal(t, "correlation", response.Header().Get(HeaderCorrelationID))
				assert.Equal(t, "lime-session", response.Header().Get(HeaderLimeSessionID))
				assert.Equal(t, "iris@example.com/server", response.Header().Get(HeaderLimeRemoteNode))
				assert.Equal(t, "bot@example.com/client", response.Header().Get(HeaderLimeLocalNode))
			} else {
				assert.Empty(t, client.incoming)
			}
		})
	}
}

func TestClientTransportReconnectResetsOldCallbackAndMuxDoesNotLeak(t *testing.T) {
	var existing http.Header
	posts := make(chan http.Header, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && existing == nil {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if request.Method == http.MethodPost {
			posts <- request.Header.Clone()
			if existing == nil || request.Header.Get(HeaderSession) == "reset" {
				existing = request.Header.Clone()
			}
		}
		for key, values := range existing {
			if strings.HasPrefix(key, HeaderSession+"-") {
				writer.Header()[key] = values
			}
		}
		writer.Header().Set(HeaderSessionID, "http-session")
		_, _ = io.WriteString(writer, `{"id":"lime-session","state":"established"}`)
	}))
	defer server.Close()
	callbackMux := NewCallbackMux()
	config := Config{BaseURL: server.URL, Identity: protocol.Identity{Name: "bot", Domain: "example.com"}, ReceiveBaseUri: "https://bot.example.com/lime", CallbackMux: callbackMux}
	first, err := NewClientTransport(config)
	require.NoError(t, err)
	defer func() { _ = first.Close() }()
	require.NoError(t, first.Send(context.Background(), &protocol.Session{State: protocol.SessionStateNew}))
	assert.Empty(t, (<-posts).Get(HeaderSession))
	require.NoError(t, first.Close())
	first.storeSessionID("late-response")
	assert.Empty(t, callbackMux.transports)

	second, err := NewClientTransport(config)
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	require.NotEqual(t, first.callbackPath, second.callbackPath)
	require.NoError(t, second.Send(context.Background(), &protocol.Session{State: protocol.SessionStateNew}))
	post := <-posts
	assert.Equal(t, "reset", post.Get(HeaderSession))
	assert.Equal(t, second.receiveBaseUri+"/messages", post.Get(HeaderSessionMessageURL))
	assert.Len(t, callbackMux.transports, 1)
	assert.Same(t, second, callbackMux.transports[second.callbackPath])
	oldResponse := httptest.NewRecorder()
	callbackMux.ServeHTTP(oldResponse, httptest.NewRequest(http.MethodPost, first.callbackPath+"/messages", nil))
	assert.Equal(t, http.StatusNotFound, oldResponse.Code)

	response := httptest.NewRecorder()
	callbackMux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, second.callbackPath+"/messages", strings.NewReader(`{"type":"text/plain","content":"new callback"}`)))
	require.Equal(t, http.StatusAccepted, response.Code)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = second.Receive(ctx)
	require.NoError(t, err)
	_, err = second.Receive(ctx)
	require.NoError(t, err)
	require.NoError(t, second.Close())
	assert.Empty(t, callbackMux.transports)
}

func TestClientTransportCanceledReceiveDoesNotConsumeQueuedEnvelope(t *testing.T) {
	client, err := NewClientTransport(Config{
		BaseURL: "https://iris.example.com", Identity: protocol.Identity{Name: "bot", Domain: "example.com"},
		ReceiveBaseUri: "https://bot.example.com/lime",
	})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	message := &protocol.Message{Envelope: protocol.Envelope{ID: "queued"}}
	require.NoError(t, client.enqueue(context.Background(), message))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Receive(ctx)
	require.ErrorIs(t, err, context.Canceled)
	envelope, err := client.Receive(context.Background())
	require.NoError(t, err)
	assert.Same(t, message, envelope)
	assert.False(t, client.polling)
}
