package main

import (
	"fmt"
	"strings"
)

// https://ashleyd.ws/custom-json-marshalling-in-golang/index.html
// https://gist.github.com/mdwhatcott/8dd2eef0042f7f1c0cd8#file-custom_json-go-L26
type Node struct {
	Identity
	Instance string
}

func (n Node) String() string {
	return fmt.Sprintf("%v/%v", n.Identity, n.Instance)
}

func ParseNode(s string) (Node, error) {
	var instance string
	values := strings.Split(s, "/")
	if len(values) > 1 {
		instance = values[1]
	}
	identity, err := ParseIdentity(values[0])
	if err != nil {
		return Node{}, err
	}

	return Node{identity, instance}, nil
}

func (n Node) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

func (n *Node) UnmarshalText(text []byte) error {
	node, err := ParseNode(string(text))
	if err != nil {
		return err
	}
	*n = node
	return nil
}
