package lime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdentityMarshalText(t *testing.T) {
	identity := Identity{
		Name:   "user",
		Domain: "example.com",
	}

	text, err := identity.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("user@example.com"), text)
}

func TestIdentityMarshalTextWithoutDomain(t *testing.T) {
	identity := Identity{
		Name: "user",
	}

	text, err := identity.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte("user"), text)
}

func TestIdentityMarshalTextEmpty(t *testing.T) {
	identity := Identity{}

	text, err := identity.MarshalText()
	assert.NoError(t, err)
	assert.Equal(t, []byte(""), text)
}

func TestIdentityUnmarshalText(t *testing.T) {
	var identity Identity

	err := identity.UnmarshalText([]byte("user@example.com"))
	assert.NoError(t, err)
	assert.Equal(t, "user", identity.Name)
	assert.Equal(t, "example.com", identity.Domain)
}

func TestIdentityUnmarshalTextWithoutDomain(t *testing.T) {
	var identity Identity

	err := identity.UnmarshalText([]byte("user"))
	assert.NoError(t, err)
	assert.Equal(t, "user", identity.Name)
	assert.Empty(t, identity.Domain)
}

func TestIdentityToNode(t *testing.T) {
	identity := Identity{
		Name:   "user",
		Domain: "example.com",
	}

	node := identity.ToNode()
	assert.Equal(t, identity, node.Identity)
	assert.Empty(t, node.Instance, "instance should be empty")
}

func TestIdentityToNodeEmpty(t *testing.T) {
	identity := Identity{}

	node := identity.ToNode()
	assert.Equal(t, identity, node.Identity)
	assert.Empty(t, node.Instance)
}
