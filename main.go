package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	session := Session{}
	session.ID = "1"
	session.To = &Node{
		Identity: Identity{"postmaster", "msging.net"},
		Instance: "#az-iris1",
	}
	session.State = SessionAuthenticating
	//session.Encryption = SessionEncryptionTLS

	session.Scheme = AuthenticationSchemePlain
	session.Authentication = &PlainAuthentication{Password: "banana"}

	b, err := json.Marshal(session)
	if err != nil {
		fmt.Println("Error", err)
	}

	fmt.Println("Session", string(b))

	identity := Identity{"postmaster", "msging.net"}

	fmt.Println("Identity", identity)

	data := []byte(`{"state": "new","id":"banana","from":"andreb@msging.net/#home"}`)

	session2 := Session{}
	err = json.Unmarshal(data, &session2)
	if err != nil {
		fmt.Println("Deserialization error:", err)
	}

	fmt.Printf("%+v\n", session2)
}
