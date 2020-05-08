package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommand_MarshalJSON_GetPingRequest(t *testing.T) {
	// Arrange
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.To = &Node{}
	c.To.Name = "postmaster"
	c.To.Domain = "limeprotocol.org"
	c.Method = CommandMethodGet
	u, _ := ParseLimeUri("/ping")
	c.Uri = &u

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"get","uri":"/ping"}`, string(b))
}

func TestCommand_UnmarshalJSON_GetPingRequest(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","method":"get","uri":"/ping"}`)
	var c Command

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", c.ID)
	assert.Nil(t, c.From)
	assert.Equal(t, &Node{Identity{"golang", "limeprotocol.org"}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	u, _ := ParseLimeUri("/ping")
	assert.Equal(t, u, *c.Uri)
	assert.Nil(t, c.Resource)
}
