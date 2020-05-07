package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

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
	assert.Equal(t, u, c.Uri)
	assert.Nil(t, c.Resource)
}
