package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	session := Session{
		Envelope: Envelope{
			ID: "1",
			To: &Node{
				Identity: Identity{Name: "postmaster", Domain: "msging.net"},
				Instance: "#az-iris1",
			},
		},
		State: SessionAuthenticating,
	}

	b, err := json.Marshal(session)
	if err != nil {
		fmt.Println("Error", err)
	}

	fmt.Println("Session", string(b))

	identity := Identity{
		Name:   "postmaster",
		Domain: "msging.net",
	}

	fmt.Println("Identity", identity)

	data := []byte(`{"state": "new","id":"banana","from":"andreb@msging.net/#home"}`)

	session2 := Session{}
	err = json.Unmarshal(data, &session2)
	if err != nil {
		fmt.Println("Deserialization error:", err)
	}

	fmt.Printf("%+v\n", session2)
}
