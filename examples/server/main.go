package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	lime "github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/server"
	"github.com/takenet/lime-go/transport"
)

func main() {
	addr := &net.TCPAddr{Port: 55321}

	srv := server.NewServerBuilder().
		MessagesHandlerFunc(
			func(ctx context.Context, msg *lime.Message, s server.Sender) error {
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v\n", msg.ID, msg.From, msg.Type)
				echoMsg := &lime.Message{}
				echoMsg.SetContent(msg.Content).SetTo(msg.From)
				return s.SendMessage(ctx, echoMsg)
			}).
		NotificationsHandlerFunc(
			func(ctx context.Context, not *lime.Notification) error {
				fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
				return nil
			}).
		AutoReplyPings().
		RequestCommandHandlerFunc(
			func(cmd *lime.RequestCommand) bool {
				return cmd.Method == lime.CommandMethodSet && cmd.URI.Path() == "/presence"
			},
			func(ctx context.Context, cmd *lime.RequestCommand, s server.Sender) error {
				return s.SendResponseCommand(
					ctx,
					cmd.SuccessResponse())
			}).
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *lime.RequestCommand, s server.Sender) error {
				fmt.Printf("RequestCommand received - ID: %v\n", cmd.ID)
				return nil
			}).
		EncryptionOptions(lime.SessionEncryptionTLS).
		ListenWebsocket(
			addr,
			&server.WebsocketConfig{
				TLSConfig: &tls.Config{
					GetCertificate: func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
						return createCertificate("localhost")
					},
				},
				TraceWriter: transport.NewStdoutTraceWriter(),
			}).
		EnableGuestAuthentication().
		EnablePlainAuthentication(func(ctx context.Context, identity lime.Identity, password string) (*server.AuthenticationResult, error) {
			if checkCredentials(identity.Name, password) {
				return &server.AuthenticationResult{Role: server.DomainRoleMember}, nil
			}
			return &server.AuthenticationResult{Role: server.DomainRoleUnknown}, nil
		}).
		Build()

	go func() {
		if err := srv.ListenAndServe(); err != server.ErrServerClosed {
			log.Printf("server: listen: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	fmt.Printf("Listening at %v. Press Ctrl+C to stop.\n", addr)
	<-sig

	if err := srv.Close(); err != nil {
		log.Printf("server: close: %v\n", err)
	}
}
