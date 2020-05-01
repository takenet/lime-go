package main

import (
	"encoding/json"
	"fmt"
	"github.com/takenet/lime-go"
)

func main() {
	session := lime.Session{}
	session.ID = "1"
	session.To = &lime.Node{
		Identity: lime.Identity{"postmaster", "msging.net"},
		Instance: "#az-iris1",
	}
	session.State = lime.SessionAuthenticating
	//session.Encryption = SessionEncryptionTLS

	session.Scheme = lime.AuthenticationSchemePlain
	session.Authentication = &lime.PlainAuthentication{Password: "banana"}

	b, err := json.Marshal(session)
	if err != nil {
		fmt.Println("Error", err)
	}

	fmt.Println("Session", string(b))

	identity := lime.Identity{"postmaster", "msging.net"}

	fmt.Println("Identity", identity)

	data := []byte(`{"state": "new","id":"banana","from":"andreb@msging.net/#home","scheme":"plain","authentication":{"password":"1234"}}`)

	session2 := lime.Session{}
	err = json.Unmarshal(data, &session2)
	if err != nil {
		fmt.Println("Deserialization error:", err)
	}

	fmt.Printf("%+v\n", session2)
}
