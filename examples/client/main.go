package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/takenet/lime-go"
	"github.com/takenet/lime-go/chat"
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
		Encryption(lime.SessionEncryptionTLS).
		UseTCP(addr, &lime.TCPConfig{
			TLSConfig:   &tls.Config{ServerName: "localhost", InsecureSkipVerify: true},
			TraceWriter: lime.NewStdoutTraceWriter(),
		}).
		Name("john").
		PlainAuthentication("mysecretpassword").
		MessagesHandlerFunc(
			func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
				return nil
			}).
		AutoReplyPings().
		RequestCommandsHandlerFunc(
			func(ctx context.Context, cmd *lime.RequestCommand, s lime.Sender) error {
				fmt.Printf("Request command received - ID: %v\n", cmd.ID)
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

	reqCmd := &lime.RequestCommand{}
	reqCmd.SetURIString("/presence").
		SetResource(&chat.Presence{
			Status:      chat.PresenceStatusAvailable,
			RoutingRule: chat.RoutingRuleIdentity}).
		SetMethod(lime.CommandMethodSet).
		SetID(lime.NewEnvelopeID())

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

		to := lime.ParseNode(toValue)

		fmt.Print("Content: ")
		scanner.Scan()
		content := lime.TextDocument(scanner.Text())

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)

		msg := &lime.Message{
			Envelope: lime.Envelope{
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
