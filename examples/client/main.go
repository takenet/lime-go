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
	"time"
)

func main() {

	addr, err := net.ResolveTCPAddr("tcp", "localhost:55321")
	if err != nil {
		log.Fatalln(err)
	}

	client := lime.NewClientBuilder().
		UseTCP(addr, &lime.TCPConfig{
			TLSConfig:   &tls.Config{ServerName: "localhost", InsecureSkipVerify: true},
			TraceWriter: lime.NewStdoutTraceWriter(),
		}).
		MessagesHandlerFunc(
			func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
				return nil
			}).
		CommandsHandlerFunc(
			func(ctx context.Context, cmd *lime.Command, s lime.Sender) error {
				fmt.Printf("Command received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
				return nil
			}).
		NotificationsHandlerFunc(
			func(ctx context.Context, not *lime.Notification) error {
				fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
				return nil
			}).
		Build()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Establish(ctx); err != nil {
		log.Fatal("Client establishment failed")
	}

	log.Println("Session established")

	presenceUri, _ := lime.ParseLimeURI("/presence")

	presence := lime.JsonDocument{
		"status":      "available",
		"routingRule": "identity",
	}

	cmd, err := client.ProcessCommand(ctx, &lime.Command{
		EnvelopeBase: lime.EnvelopeBase{
			ID: lime.NewEnvelopeId(),
			To: lime.Node{
				Identity: lime.Identity{Name: "postmaster", Domain: "msging.net"},
				Instance: "",
			},
		},
		Method: lime.CommandMethodSet,
		URI:    &presenceUri,
		Type: &lime.MediaType{
			Type:    "application",
			Subtype: "vnd.lime.presence",
			Suffix:  "json",
		},
		Resource: &presence,
	})
	if err != nil {
		log.Fatalln(err)
	}

	if cmd != nil {
		log.Printf("Command response received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
	}

	ctx, cancel = context.WithCancel(context.Background())

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	fmt.Println("Press Ctrl+C key to exit")
	<-sig

	cancel()

	err = client.Close()
	if err != nil {
		log.Fatalln(err)
	}
}
