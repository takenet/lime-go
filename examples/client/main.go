package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/takenet/lime-go/client"
	"github.com/takenet/lime-go/protocol"
	"github.com/takenet/lime-go/resources"
)

func main() {
	client := client.NewClientBuilder().
		Encryption(protocol.SessionEncryptionTLS).
		UseWebsocket("wss://localhost:55321", nil, &tls.Config{ServerName: "localhost", InsecureSkipVerify: true}).
		Name("john").
		PlainAuthentication("mysecretpassword").
		MessagesHandlerFunc(
			func(ctx context.Context, msg *protocol.Message, s client.Sender) error {
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
				return nil
			}).
		AutoReplyPings().
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *protocol.RequestCommand, s client.Sender) error {
				fmt.Printf("Request command received - ID: %v\n", cmd.ID)
				return nil
			}).
		NotificationsHandlerFunc(
			func(ctx context.Context, not *protocol.Notification) error {
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

	reqCmd := &protocol.RequestCommand{}
	reqCmd.SetURIString("/presence").
		SetResource(&resources.Presence{
			Status:      resources.PresenceStatusAvailable,
			RoutingRule: resources.RoutingRuleIdentity}).
		SetMethod(protocol.CommandMethodSet).
		SetID(protocol.NewEnvelopeID())

	respCmd, err := client.ProcessCommand(ctx, reqCmd)
	if err != nil {
		log.Fatalln(err)
	}

	if respCmd != nil {
		log.Printf("Command response received - ID: %v - Status: %v\n", respCmd.ID, respCmd.Status)
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("To: ")
		scanner.Scan()

		toValue := scanner.Text()
		if toValue == "" || toValue == "exit" || toValue == "quit" {
			break
		}

		to := protocol.ParseNode(toValue)

		fmt.Print("Content: ")
		scanner.Scan()
		content := protocol.TextDocument(scanner.Text())

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

		msg := &protocol.Message{
			Envelope: protocol.Envelope{
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
