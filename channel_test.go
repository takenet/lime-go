package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestChannel_MarshalJSON_GetPingRequest(t *testing.T) {
	// Arrange

	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.To = Node{}
	c.To.Name = "postmaster"
	c.To.Domain = "limeprotocol.org"
	c.Method = CommandMethodGet
	u, _ := ParseLimeUri("/ping")
	c.Uri = &u

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"get","uri":"/ping"}`, string(b))
}
