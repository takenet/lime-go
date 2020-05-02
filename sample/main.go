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
		Identity: lime.Identity{Name: "postmaster", Domain: "msging.net"},
		Instance: "#az-iris1",
	}
	session.State = lime.SessionStateAuthenticating
	//session.Encryption = SessionEncryptionTLS

	//session.Scheme = lime.AuthenticationSchemePlain
	session.SetAuthentication(&lime.PlainAuthentication{Password: "banana"})
	//session.SetAuthentication(&lime.PlainAuthentication{Password: "banana"})

	b, err := json.Marshal(session)
	if err != nil {
		fmt.Println("Error", err)
	}

	fmt.Println("Session", string(b))

	identity := lime.Identity{"postmaster", "msging.net"}

	fmt.Println("Identity", identity)

	data := []byte(`{"state": "new","id":"banana","from":"andreb@msging.net/#home","scheme":"plain","authentication":{"password":"1234"},"reason":{"code":1,"description":"crap"}}`)

	session2 := lime.Session{}
	err = json.Unmarshal(data, &session2)
	if err != nil {
		fmt.Println("Deserialization error:", err)
	}

	fmt.Printf("%+v\n", session2)

	//auth, err := session2.GetAuthentication()
	//fmt.Printf("authentication: %+v\n", auth)
}
