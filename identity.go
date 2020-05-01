package lime

import (
	"fmt"
	"strings"
)

type Identity struct {
	Name   string
	Domain string
}

func (i Identity) String() string {
	if i == (Identity{}) {
		return ""
	}

	return fmt.Sprintf("%v@%v", i.Name, i.Domain)
}

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
