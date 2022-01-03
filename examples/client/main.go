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

	config := &lime.TCPConfig{
		TLSConfig:   &tls.Config{ServerName: "localhost", InsecureSkipVerify: true},
		TraceWriter: lime.NewStdoutTraceWriter(),
	}

	t, err := lime.DialTcp(context.Background(), addr, config)

	if err != nil {
		log.Fatalln(err)
	}

	client := lime.NewClientChannel(t, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	ses, err := client.EstablishSession(
		ctx,
		func(compressions []lime.SessionCompression) lime.SessionCompression {
			return lime.SessionCompressionNone
		},
		func(encryptions []lime.SessionEncryption) lime.SessionEncryption {
			return lime.SessionEncryptionTLS
		},
		lime.Identity{
			Name:   lime.NewEnvelopeId(),
			Domain: "localhost",
		},
		func(schemes []lime.AuthenticationScheme, authentication lime.Authentication) lime.Authentication {
			return &lime.GuestAuthentication{}
		},
		"default",
	)

	cancel()

	if err != nil {
		log.Fatalln(err)
	}

	if ses.State != lime.SessionStateEstablished {
		fmt.Printf("The session was not established - ID: %v - State: %v\n - Reason: %v", ses.ID, ses.State, ses.Reason)
	}

	fmt.Println("Session established")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
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
		fmt.Printf("Command response received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
	}

	ctx, cancel = context.WithCancel(context.Background())

	mux := lime.EnvelopeMux{}
	mux.MessageHandlerFunc(
		func(*lime.Message) bool {
			return true
		},
		func(ctx context.Context, msg *lime.Message, s lime.Sender) error {
			fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
			return nil
		})
	mux.CommandHandlerFunc(
		func(*lime.Command) bool {
			return true
		},
		func(ctx context.Context, cmd *lime.Command, s lime.Sender) error {
			fmt.Printf("Command received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
			return nil
		})
	mux.NotificationHandlerFunc(
		func(*lime.Notification) bool {
			return true
		},
		func(ctx context.Context, not *lime.Notification) error {
			fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
			return nil
		})

	go func() {
		if err := mux.ListenClient(ctx, client); err != nil {
			fmt.Printf("Listener error: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig)
	fmt.Println("Press Ctrl+C key to exit")
	<-sig

	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	ses, err = client.FinishSession(ctx)

	cancel()

	if err != nil {
		log.Fatalln(err)
	}

	if ses.State != lime.SessionStateFinished {
		fmt.Printf("The session was not finished - ID: %v - State: %v\n - Reason: %v", ses.ID, ses.State, ses.Reason)
	}
}
