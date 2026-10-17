package address

import (
	"fmt"
	"strings"
)

// Node represents an identity and optional instance on the LIME network.
type Node struct {
	Identity
	Instance string
}

func (node Node) String() string {
	if node == (Node{}) {
		return ""
	}
	if node.Instance == "" {
		return node.Identity.String()
	}
	return fmt.Sprintf("%v/%v", node.Identity, node.Instance)
}

// ParseNode parses a LIME node in name@domain/instance form.
func ParseNode(value string) Node {
	parts := strings.Split(value, "/")
	node := Node{Identity: ParseIdentity(parts[0])}
	if len(parts) > 1 {
		node.Instance = parts[1]
	}
	return node
}

func (node Node) MarshalText() ([]byte, error) {
	return []byte(node.String()), nil
}

func (node *Node) UnmarshalText(value []byte) error {
	*node = ParseNode(string(value))
	return nil
}

// IsComplete reports whether the identity and instance are present.
func (node *Node) IsComplete() bool {
	return node != nil && node.Identity.IsComplete() && node.Instance != ""
}
