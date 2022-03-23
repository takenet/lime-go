package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/takenet/lime-go"
	"log"
	"net"
	"os"
	"os/signal"
)

func main() {
	addr := &net.TCPAddr{Port: 55321}

	server := lime.NewServerBuilder().
		MessagesHandlerFunc(
			func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v\n", msg.ID, msg.From, msg.Type)
				return s.SendMessage(ctx, &lime.Message{
					EnvelopeBase: lime.EnvelopeBase{
						To: msg.From,
					},
					Type:    msg.Type,
					Content: msg.Content,
				})
			}).
		NotificationsHandlerFunc(
			func(ctx context.Context, not *lime.Notification) error {
				fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
				return nil
			}).
		RequestCommandHandlerFunc(
			func(cmd *lime.RequestCommand) bool {
				if cmd.URI == nil {
					return false
				}

				url := cmd.URI.ToURL()
				return url.String() == "/presence"
			},
			func(ctx context.Context, cmd *lime.RequestCommand, s lime.Sender) error {
				return s.SendResponseCommand(
					ctx,
					cmd.SuccessResponse())
			}).
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *lime.RequestCommand, s lime.Sender) error {
				fmt.Printf("RequestCommand received - ID: %v\n", cmd.ID)
				return nil
			}).
		ListenTCP(
			addr,
			&lime.TCPConfig{
				TLSConfig: &tls.Config{
					GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
						return createCertificate("localhost")
					},
				},
				TraceWriter: lime.NewStdoutTraceWriter(),
			}).
		EnableGuestAuthentication().
		Build()

	go func() {
		if err := server.ListenAndServe(); err != lime.ErrServerClosed {
			log.Printf("server: listen: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	fmt.Printf("Listening at %v. Press Ctrl+C to stop.\n", addr)
	<-sig

	if err := server.Close(); err != nil {
		log.Printf("server: close: %v\n", err)
	}
}
