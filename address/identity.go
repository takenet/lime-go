package address

import (
	"fmt"
	"strings"
)

// Identity represents a member of a domain.
type Identity struct {
	Name   string
	Domain string
}

func (identity Identity) String() string {
	if identity == (Identity{}) {
		return ""
	}
	if identity.Domain == "" {
		return identity.Name
	}
	return fmt.Sprintf("%v@%v", identity.Name, identity.Domain)
}

// ParseIdentity parses a LIME identity in name@domain form.
func ParseIdentity(value string) Identity {
	parts := strings.Split(value, "@")
	identity := Identity{Name: parts[0]}
	if len(parts) > 1 {
		identity.Domain = parts[1]
	}
	return identity
}

func (identity Identity) MarshalText() ([]byte, error) {
	return []byte(identity.String()), nil
}

func (identity *Identity) UnmarshalText(value []byte) error {
	*identity = ParseIdentity(string(value))
	return nil
}

// ToNode creates a node with no instance component.
func (identity Identity) ToNode() Node {
	return Node{Identity: identity}
}

// IsComplete reports whether both identity components are present.
func (identity *Identity) IsComplete() bool {
	return identity != nil && identity.Name != "" && identity.Domain != ""
}
