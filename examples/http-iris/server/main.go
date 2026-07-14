// Command server demonstrates receiving LIME envelopes forwarded by Iris
// (Blip's HTTP gateway) as webhook requests, using the
// github.com/takenet/lime-go/http package.
//
// Usage:
//
//	IRIS_KEY=<access-key> go run ./examples/http-iris/server
//
// Configure the resulting HTTPS endpoint as the bot's webhook URL in Blip's
// portal so that Iris starts forwarding envelopes to it.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/takenet/lime-go"
	limehttp "github.com/takenet/lime-go/http"
)

func main() {
	accessKey := os.Getenv("IRIS_KEY")

	handler := &limehttp.Handler{
		Authenticator: limehttp.AuthenticatorFunc(
			func(ctx context.Context, identity lime.Identity, scheme lime.AuthenticationScheme, credential string) (bool, error) {
				// In production, validate the identity/credential pair against
				// your own access key store instead of a single shared secret.
				return credential == accessKey, nil
			}),
		MessageHandler: func(ctx context.Context, msg *lime.Message) (*lime.Notification, error) {
			log.Printf("message received - ID: %v - From: %v - Content: %v\n", msg.ID, msg.From, msg.Content)
			return msg.Notification(lime.NotificationEventConsumed), nil
		},
		NotificationHandler: func(ctx context.Context, not *lime.Notification) error {
			log.Printf("notification received - ID: %v - From: %v - Event: %v\n", not.ID, not.From, not.Event)
			return nil
		},
		CommandHandler: func(ctx context.Context, cmd *lime.RequestCommand) (*lime.ResponseCommand, error) {
			log.Printf("command received - ID: %v - Method: %v\n", cmd.ID, cmd.Method)
			return cmd.SuccessResponse(), nil
		},
	}

	log.Println("Listening for Iris webhooks on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}
