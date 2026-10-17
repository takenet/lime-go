package channel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvelopeMuxDefaults(t *testing.T) {
	for _, kind := range []string{"message", "notification", "request"} {
		t.Run(kind, func(t *testing.T) {
			for _, scenario := range []string{"explicit", "fallback", "unmatched", "error", "fallback error"} {
				t.Run(scenario, func(t *testing.T) {
					mux := &EnvelopeMux{}
					var calls []string
					failure := errors.New("handler failed")
					register := func(fallback bool, match bool, name string) {
						invoke := func() error {
							calls = append(calls, name)
							if scenario == "error" || scenario == "fallback error" {
								return failure
							}
							return nil
						}
						switch kind {
						case "message":
							add := mux.MessageHandlerFunc
							if fallback {
								add = mux.DefaultMessageHandlerFunc
							}
							add(func(*Message) bool { return match }, func(context.Context, *Message, Sender) error { return invoke() })
						case "notification":
							add := mux.NotificationHandlerFunc
							if fallback {
								add = mux.DefaultNotificationHandlerFunc
							}
							add(func(*Notification) bool { return match }, func(context.Context, *Notification) error { return invoke() })
						case "request":
							add := mux.RequestCommandHandlerFunc
							if fallback {
								add = mux.DefaultRequestCommandHandlerFunc
							}
							add(func(*RequestCommand) bool { return match }, func(context.Context, *RequestCommand, Sender) error { return invoke() })
						}
					}
					register(true, false, "skipped default")
					register(true, scenario != "unmatched", "default")
					register(false, false, "skipped explicit")
					register(false, scenario == "explicit" || scenario == "error", "explicit")
					register(true, scenario != "unmatched", "later default")
					register(false, scenario == "explicit" || scenario == "error", "later explicit")
					counts := mux.HandlerCounts()
					require.Equal(t, 6, counts.Messages+counts.Notifications+counts.RequestCommands)
					var err error
					switch kind {
					case "message":
						err = mux.handleMessage(context.Background(), &Message{}, nil)
					case "notification":
						err = mux.handleNotification(context.Background(), &Notification{})
					case "request":
						err = mux.handleRequestCommand(context.Background(), &RequestCommand{}, nil)
					}
					if scenario == "error" || scenario == "fallback error" {
						require.ErrorIs(t, err, failure)
					} else {
						require.NoError(t, err)
					}
					switch scenario {
					case "explicit", "error":
						require.Equal(t, []string{"explicit"}, calls)
					case "fallback", "fallback error":
						require.Equal(t, []string{"default"}, calls)
					case "unmatched":
						require.Empty(t, calls)
					}
				})
			}
		})
	}
}
