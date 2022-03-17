package main

import (
	"context"
	"github.com/takenet/lime-go"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
)

func main() {
	wsConfig := &lime.WebsocketConfig{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	server := lime.NewServerBuilder().
		MessagesHandlerFunc(
			func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
				log.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
				return s.SendMessage(ctx, &lime.Message{
					EnvelopeBase: lime.EnvelopeBase{
						To: msg.From,
					},
					Type:    msg.Type,
					Content: msg.Content,
				})
				return nil
			}).
		NotificationsHandlerFunc(
			func(ctx context.Context, not *lime.Notification) error {
				log.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
				return nil
			}).
		CommandHandlerFunc(
			func(cmd *lime.Command) bool {
				if cmd.Status != "" || cmd.URI == nil {
					return false
				}

				url := cmd.URI.ToURL()
				return cmd.Status == "" &&
					url.String() == "/presence"
			},
			func(ctx context.Context, cmd *lime.Command, s lime.Sender) error {
				return s.SendCommand(
					ctx,
					cmd.SuccessResponse())
			}).
		CommandsHandlerFunc(
			func(ctx context.Context, cmd *lime.Command, s lime.Sender) error {
				log.Printf("Command received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
				return nil
			}).
		ListenWebsocket(&net.TCPAddr{Port: 8080}, wsConfig).
		Build()

	go func() {
		if err := server.ListenAndServe(); err != lime.ErrServerClosed {
			log.Printf("server: listen: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	log.Println("Listening at ws:8080. Press Ctrl+C to stop.")
	<-sig

	if err := server.Close(); err != nil {
		log.Printf("server: close: %v\n", err)
	}
}
