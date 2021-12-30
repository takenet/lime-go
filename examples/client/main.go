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
	go func() {
		for client.Established() {
			select {
			case <-ctx.Done():
				fmt.Println("Listener stopped")
				return
			case msg := <-client.MsgChan():
				fmt.Printf("Message received - ID: %v - From: %v - Type: %v - Content: %v\n", msg.ID, msg.From, msg.Type, msg.Content)
			case not := <-client.NotChan():
				fmt.Printf("Notification received - ID: %v - From: %v - Event: %v - Reason: %v\n", not.ID, not.From, not.Event, not.Reason)
			case cmd := <-client.CmdChan():
				fmt.Printf("Command received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
			}
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
