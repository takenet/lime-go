package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/protocol"
	httptransport "github.com/takenet/lime-go/transport/http"
	"go.uber.org/goleak"
)

func TestClientHTTPSessionRecoveryDoesNotReplayMessages(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, sendSessionID := range []bool{false, true} {
		t.Run(fmt.Sprintf("session_id=%t", sendSessionID), func(t *testing.T) {
			var preflights, sessions, messages atomic.Int32
			callbackURLs := make(chan string, 4)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				assert.Equal(t, "bot@example.com", request.Header.Get(httptransport.HeaderSessionIdentity))
				switch request.URL.Path {
				case "/sessions":
					if request.Method == http.MethodGet {
						preflights.Add(1)
						assert.Empty(t, request.Header.Get(httptransport.HeaderSessionID))
						writer.WriteHeader(http.StatusNotFound)
						return
					}
					session := sessions.Add(1)
					assert.Empty(t, request.Header.Get(httptransport.HeaderSessionID))
					callbackURLs <- request.Header.Get(httptransport.HeaderSessionNotificationURL)
					writer.Header().Set(httptransport.HeaderSessionID, fmt.Sprintf("http-%d", session))
					_, _ = fmt.Fprintf(writer, `{"id":"lime-%d","state":"established","to":"bot@example.com/iris-%d"}`, session, session)
				case "/messages":
					message := messages.Add(1)
					expectedID := ""
					if sendSessionID {
						expectedID = "http-1"
						if message > 1 {
							expectedID = "http-2"
						}
					}
					assert.Equal(t, expectedID, request.Header.Get(httptransport.HeaderSessionID))
					if message == 1 {
						writer.WriteHeader(http.StatusUnauthorized)
						_, _ = io.WriteString(writer, `{"code":4,"description":"Invalid HTTP session id"}`)
						return
					}
					writer.WriteHeader(http.StatusAccepted)
				default:
					http.NotFound(writer, request)
				}
			}))
			defer server.Close()

			callbackMux := httptransport.NewCallbackMux()
			received := make(chan *protocol.Notification, 1)
			connection := NewClientBuilder().Name("bot").Domain("example.com").
				UseHTTPTransport(httptransport.Config{
					BaseURL: server.URL, ReceiveBaseUri: "https://bot.example.com/lime",
					CallbackMux: callbackMux, SendSessionIDHeader: sendSessionID,
				}).
				NotificationsHandlerFunc(func(_ context.Context, notification *protocol.Notification) error {
					received <- notification
					return nil
				}).Build()
			defer func() {
				connection.stopListener()
				if active := connection.establishedChannel(); active != nil {
					require.NoError(t, active.Close())
				}
				require.NoError(t, connection.Close())
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			require.NoError(t, connection.Establish(ctx))
			first := connection.establishedChannel()
			require.NotNil(t, first)
			firstCallback := <-callbackURLs
			message := &protocol.Message{Envelope: protocol.Envelope{ID: "failed-send"}}
			content := protocol.TextDocument("hello")
			message.SetContent(&content)
			require.ErrorContains(t, connection.SendMessage(ctx, message), "returned status 401")
			require.False(t, first.Established())
			require.Eventually(t, func() bool {
				current := connection.establishedChannel()
				return current != nil && current != first
			}, time.Second, time.Millisecond, "the listener must recover without another application request")
			require.NotSame(t, first, connection.establishedChannel())
			require.EqualValues(t, 2, sessions.Load())
			require.EqualValues(t, 2, preflights.Load())
			require.EqualValues(t, 1, messages.Load(), "recovery must not replay the failed message")
			secondCallback := <-callbackURLs
			require.NotEqual(t, firstCallback, secondCallback)

			for range 3 {
				require.NoError(t, connection.SendMessage(ctx, &protocol.Message{
					Envelope: protocol.Envelope{ID: "new-send"}, Content: message.Content,
				}))
			}
			require.EqualValues(t, 2, sessions.Load(), "business sends must reuse the recovered session")
			require.EqualValues(t, 2, preflights.Load())
			require.EqualValues(t, 4, messages.Load())
			oldResponse := httptest.NewRecorder()
			callbackMux.ServeHTTP(oldResponse, httptest.NewRequest(http.MethodPost, firstCallback, strings.NewReader(`{"id":"new-send","event":"failed"}`)))
			require.Equal(t, http.StatusNotFound, oldResponse.Code)
			response := httptest.NewRecorder()
			callbackMux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, secondCallback, strings.NewReader(`{"id":"new-send","event":"failed"}`)))
			require.Equal(t, http.StatusAccepted, response.Code)
			select {
			case notification := <-received:
				require.Equal(t, protocol.NotificationEventFailed, notification.Event)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
