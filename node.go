package lime

import (
	"fmt"
	"strings"
)

// Node represents an element of a network.
type Node struct {
	Identity
	// Instance represents the name of the instance used by the node to connect to the network.
	Instance string
}

func (n Node) String() string {
	if n == (Node{}) {
		return ""
	}

	if n.Instance == "" {
		return n.Identity.String()
	}

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

// IsComplete indicates if all Node fields has values.
func (n *Node) IsComplete() bool {
	return n.Identity.IsComplete() && n.Instance != ""
}
