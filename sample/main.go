package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/takenet/lime-go"
	"net"
	"os"
)

func main() {

	sender := func(t lime.Transport, e lime.Envelope) error {
		err := t.Send(context.Background(), e)
		if err != nil {
			fmt.Println("Send error:", err)
			return err
		}
		return nil
	}

	receiver := func(t lime.Transport) (lime.Envelope, error) {
		e, err := t.Receive(context.TODO())
		if err != nil {
			fmt.Println("Receive error:", err)
			return nil, err
		}
		return e, nil
	}

	whileNotError := func(fs ...func() error) error {
		for _, f := range fs {
			err := f()
			if err != nil {
				return err
			}
		}
		return nil
	}

	t := lime.TCPTransport{}
	t.TLSConfig = &tls.Config{ServerName: "msging.net"}

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
	var sessionId string

	_ = whileNotError(
		func() error {
			return sender(&t, &lime.Session{State: lime.SessionStateNew})
		},
		func() error {
			// Negotiating
			e, err := receiver(&t)
			if e != nil {
				sessionId = e.GetID()
			}
			return err
		},
		func() error {
			s := lime.Session{}
			s.ID = sessionId
			s.State = lime.SessionStateNegotiating
			s.Compression = lime.SessionCompressionNone
			s.Encryption = lime.SessionEncryptionTLS
			return sender(&t, &s)
		},
		func() error {
			// Negotiation ack
			_, err := receiver(&t)
			err = t.SetEncryption(context.Background(), lime.SessionEncryptionTLS)
			return err
		},
		func() error {
			// Authenticating
			_, err := receiver(&t)
			return err
		},
		func() error {
			s := lime.Session{}
			s.ID = sessionId
			s.From = lime.Node{
				Identity: lime.Identity{
					Name:   "andreb",
					Domain: "msging.net",
				},
				Instance: "default",
			}

			s.State = lime.SessionStateAuthenticating
			auth := lime.PlainAuthentication{}
			auth.SetPasswordAsBase64("123456")
			s.SetAuthentication(&auth)
			return sender(&t, &s)
		},
		func() error {
			// Established
			s, err := receiver(&t)
			if s != nil {
				fmt.Printf("Session %v established\n", s.GetID())
			}
			return err
		})

	fmt.Println("Press ENTER key to exit")
	reader := bufio.NewReader(os.Stdin)
	reader.ReadString('\n')

	_ = sender(&t, &lime.Session{EnvelopeBase: lime.EnvelopeBase{ID: sessionId}, State: lime.SessionStateFinishing})
	receiver(&t)

	//s := lime.Session{State: lime.SessionStateNew}
	//err = t.Send(&s)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//
	//b, err := json.Marshal(&s)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println("Send: ", string(b))
	//
	//e, err := t.Receive()
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//
	//b, err = json.Marshal(&e)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//fmt.Println("Session", string(b))

	//conn, err := net.Dial("tcp", "tcp.msging.net:443")
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}
	//
	//enc := json.NewEncoder(conn)
	//dec := json.NewDecoder(conn)
	//err = enc.Encode(lime.Session{State: lime.SessionStateNew})
	//
	//var receivedSession lime.EnvelopeUnion
	//err = dec.Decode(&receivedSession)
	//if err != nil {
	//	fmt.Println(err)
	//	return
	//}

	//session := lime.Session{}
	//session.ID = "1"
	//session.To = lime.Node{
	//	Identity: lime.Identity{Name: "postmaster", Domain: "msging.net"},
	//	Instance: "#az-iris1",
	//}
	//session.State = lime.SessionStateAuthenticating
	////session.Encryption = SessionEncryptionTLS
	//
	////session.Scheme = lime.AuthenticationSchemePlain
	//session.SetAuthentication(&lime.PlainAuthentication{Password: "banana"})
	////session.SetAuthentication(&lime.PlainAuthentication{Password: "banana"})
	//
	//b, err := json.Marshal(session)
	//if err != nil {
	//	fmt.Println("Error", err)
	//}
	//
	//fmt.Println("Session", string(b))
	//
	//identity := lime.Identity{"postmaster", "msging.net"}
	//
	//fmt.Println("Identity", identity)
	//
	//data := []byte(`{"state": "new","id":"banana","from":"andreb@msging.net/#home","scheme":"plain","authentication":{"password":"1234"},"reason":{"code":1,"description":"crap"}}`)
	//
	//session2 := lime.Session{}
	//err = json.Unmarshal(data, &session2)
	//if err != nil {
	//	fmt.Println("Deserialization error:", err)
	//}
	//
	//fmt.Printf("%+v\n", session2)
	//
	////auth, err := session2.GetAuthentication()
	////fmt.Printf("authentication: %+v\n", auth)
}

func listen() {
	listener := lime.TCPTransportListener{}

	addr, err := net.ResolveTCPAddr("tcp", ":55321")
	if err != nil {
		fmt.Println(err)
	}

	err = listener.Open(context.Background(), addr)
	if err != nil {
		fmt.Println("Listener error:", err)
	}

	for {
		t, err := listener.Accept()
		if err != nil {
			fmt.Println(err)
		}

		err = t.Send(context.Background(), &lime.Session{State: lime.SessionStateFailed})
		if err != nil {
			fmt.Println(err)
		}
		_ = t.Close(context.TODO())
	}
}
