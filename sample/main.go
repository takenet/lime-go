package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/takenet/lime-go"
	"net"
	"os"
	"time"
)

func main() {

	t := lime.TCPTransport{
		TLSConfig: &tls.Config{ServerName: "msging.net"},
	}
	tw := lime.NewStdoutTraceWriter()
	t.TraceWriter = tw

	addr, err := net.ResolveTCPAddr("tcp", "tcp.msging.net:443")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = t.Open(context.Background(), addr)
	if err != nil {
		fmt.Println(err)
		return
	}

	client, err := lime.NewClientChannel(&t, 1)
	if err != nil {
		fmt.Println(err)
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	ses, err := client.EstablishSession(
		ctx,
		func(compressions []lime.SessionCompression) lime.SessionCompression {
			return lime.SessionCompressionNone
		},
		func(encryptions []lime.SessionEncryption) lime.SessionEncryption {
			return lime.SessionEncryptionTLS
		},
		lime.Identity{
			Name:   "andreb",
			Domain: "msging.net",
		},
		func(schemes []lime.AuthenticationScheme, authentication lime.Authentication) lime.Authentication {
			auth := lime.PlainAuthentication{}
			auth.SetPasswordAsBase64("123456")
			return &auth
		},
		"default",
	)

	if err != nil {
		fmt.Println(err)
		return
	}

	if ses.State != lime.SessionStateEstablished {
		fmt.Printf("The session was not established - ID: %v - State: %v\n - Reason: %v", ses.ID, ses.State, ses.Reason)
	}

	fmt.Println("Session established")

	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second)
	ping, _ := lime.ParseLimeUri("/ping")
	cmd, err := client.ProcessCommand(ctx, &lime.Command{
		EnvelopeBase: lime.EnvelopeBase{
			ID: lime.NewEnvelopeId(),
			To: lime.Node{
				Identity: lime.Identity{Name: "postmaster", Domain: "msging.net"},
				Instance: "",
			},
		},
		Method: lime.CommandMethodGet,
		Uri:    &ping,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	if cmd != nil {
		fmt.Printf("Command response received - ID: %v - Status: %v\n", cmd.ID, cmd.Status)
	}

	fmt.Println("Press ENTER key to exit")

	reader := bufio.NewReader(os.Stdin)
	_, err = reader.ReadString('\n')

	ctx, _ = context.WithTimeout(context.Background(), 5*time.Second)
	ses, err = client.FinishSession(ctx)

	if err != nil {
		fmt.Println(err)
		return
	}

	if ses.State != lime.SessionStateFinished {
		fmt.Printf("The session was not finished - ID: %v - State: %v\n - Reason: %v", ses.ID, ses.State, ses.Reason)
	}
}

//sender := func(t lime.Transport, e lime.Envelope) error {
//	err := t.Send(context.Background(), e)
//	if err != nil {
//		fmt.Println("Send error:", err)
//		return err
//	}
//	return nil
//}
//
//receiver := func(t lime.Transport) (lime.Envelope, error) {
//	e, err := t.Receive(context.TODO())
//	if err != nil {
//		fmt.Println("Receive error:", err)
//		return nil, err
//	}
//	return e, nil
//}
//
//whileNotError := func(fs ...func() error) error {
//	for _, f := range fs {
//		err := f()
//		if err != nil {
//			return err
//		}
//	}
//	return nil
//}
//
//var sessionId string
//
//err = whileNotError(
//	func() error {
//		return sender(&t, &lime.Session{State: lime.SessionStateNew})
//	},
//	func() error {
//		// Negotiating
//		e, err := receiver(&t)
//		if e != nil {
//			sessionId = e.GetID()
//		}
//		return err
//	},
//	func() error {
//		s := lime.Session{}
//		s.ID = sessionId
//		s.State = lime.SessionStateNegotiating
//		s.Compression = lime.SessionCompressionNone
//		s.Encryption = lime.SessionEncryptionTLS
//		return sender(&t, &s)
//	},
//	func() error {
//		// Negotiation ack
//		_, err := receiver(&t)
//		err = t.SetEncryption(context.Background(), lime.SessionEncryptionTLS)
//		return err
//	},
//	func() error {
//		// Authenticating
//		_, err := receiver(&t)
//		return err
//	},
//	func() error {
//		s := lime.Session{}
//		s.ID = sessionId
//		s.From = lime.Node{
//			Identity: lime.Identity{
//				Name:   "andreb",
//				Domain: "msging.net",
//			},
//			Instance: "default",
//		}
//
//		s.State = lime.SessionStateAuthenticating
//		auth := lime.PlainAuthentication{}
//		auth.SetPasswordAsBase64("123456")
//		s.SetAuthentication(&auth)
//		return sender(&t, &s)
//	},
//	func() error {
//		// Established
//		s, err := receiver(&t)
//		if s != nil {
//			fmt.Printf("Session %v established\n", s.GetID())
//		}
//		return err
//	})

//
//err = sender(&t, &lime.Session{EnvelopeBase: lime.EnvelopeBase{ID: sessionId}, State: lime.SessionStateFinishing})
//_, err = receiver(&t)
