package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/takenet/lime-go"
	"net"
)

func main() {

	sender := func(t lime.Transport, e lime.Envelope) error {
		err := t.Send(e)
		if err != nil {
			fmt.Println("Send error:", err)
			return err
		}
		b, err := json.Marshal(e)
		if err != nil {
			fmt.Println("Marshalling error:", err)
			return err
		}
		fmt.Println("Send:", string(b))
		return nil
	}

	receiver := func(t lime.Transport) error {
		e, err := t.Receive()
		if err != nil {
			fmt.Println("Receive error:", err)
			return err
		}
		b, err := json.Marshal(e)
		if err != nil {
			fmt.Println("Marshalling error:", err)
			return err
		}
		fmt.Println("Receive:", string(b))
		return nil
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

	t := lime.TcpTransport{}

	add, err := net.ResolveTCPAddr("tcp", "tcp.msging.net:443")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = t.Open(context.Background(), add)
	if err != nil {
		fmt.Println(err)
		return
	}

	_ = whileNotError(
		func() error {
			return sender(&t, &lime.Session{State: lime.SessionStateNew})
		},
		func() error {
			return receiver(&t)
		},
		func() error {
			return sender(&t, &lime.Session{})
		})

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
