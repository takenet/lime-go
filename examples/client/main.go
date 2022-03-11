package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/takenet/lime-go"
	"log"
	"net"
	"os"
	"time"
)

func main() {

	addr, err := net.ResolveTCPAddr("tcp", "localhost:55321")
	if err != nil {
		log.Fatalf("Client establishment failed: %v", err)
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
		log.Fatalf("Client establishment failed: %v", err)
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

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("To: ")
		scanner.Scan()

		to, err := lime.ParseNode(scanner.Text())
		if err != nil {
			fmt.Printf("Invalid node: %v\n", err)
			continue
		}

		fmt.Print("Content: ")
		scanner.Scan()
		content := lime.PlainDocument(scanner.Text())

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

		msg := &lime.Message{
			EnvelopeBase: lime.EnvelopeBase{
				To: to,
			},
		}
		msg.SetContent(content)

		if err := client.SendMessage(ctx, msg); err != nil {
			fmt.Printf("Send message error: %v\n", err)
		}
		cancel()
	}

	err = client.Close()
	if err != nil {
		log.Fatalln(err)
	}
}
