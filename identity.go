package lime

import (
	"fmt"
	"strings"
)

// Identity represents a member of a domain.
type Identity struct {
	// Name represents the Identity unique name on its domain.
	Name string
	// Domain represents the network domain name of the Identity.
	Domain string
}

func (i Identity) String() string {
	if i == (Identity{}) {
		return ""
	}

	if i.Domain == "" {
		return i.Name
	}

	return fmt.Sprintf("%v@%v", i.Name, i.Domain)
}

// ParseIdentity parses the string To a valid Identity.
func ParseIdentity(s string) Identity {
	var name, domain string
	values := strings.Split(s, "@")
	if len(values) > 1 {
		domain = values[1]
	}
	name = values[0]
	return Identity{name, domain}
}

func (i Identity) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

func (i *Identity) UnmarshalText(text []byte) error {
	identity := ParseIdentity(string(text))
	*i = identity
	return nil
}

// ToNode creates a Node instance based on the identity, with an
// empty value for the instance property.
func (i Identity) ToNode() Node {
	return Node{i, ""}
}

// IsComplete indicates if all Identity fields has values.
func (i *Identity) IsComplete() bool {
	return i.Name != "" && i.Domain != ""
}
