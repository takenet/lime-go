package lime

import (
	"fmt"
	"strings"
)

// Represents a member of a domain.
type Identity struct {
	// Identity unique name on his domain.
	Name string
	// Network domain name of the identity.
	Domain string
}

func (i Identity) String() string {
	if i == (Identity{}) {
		return ""
	}

	return fmt.Sprintf("%v@%v", i.Name, i.Domain)
}

// Parses the string to a valid Identity.
func ParseIdentity(s string) (Identity, error) {
	var name, domain string
	values := strings.Split(s, "@")
	if len(values) > 1 {
		domain = values[1]
	}
	name = values[0]
	return Identity{name, domain}, nil
}

func (i Identity) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

func (i *Identity) UnmarshalText(text []byte) error {
	identity, err := ParseIdentity(string(text))
	if err != nil {
		return err
	}
	*i = identity
	return nil
}

// Creates a Node instance based on the identity, with a
// empty value for the instance property.
func (i Identity) ToNode() Node {
	return Node{i, ""}
}
