package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMessage_MarshalJSON_Plain(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	m.To = &Node{}
	m.To.Name = "golang"
	m.To.Domain = "limeprotocol.org"
	m.To.Instance = "default"
	var d PlainDocument = "Hello world"
	m.SetContent(&d)

	// Act
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world"}`, string(b))
}

func TestMessage_MarshalJSON_Json(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	m.To = &Node{}
	m.To.Name = "golang"
	m.To.Domain = "limeprotocol.org"
	m.To.Instance = "default"
	d := JsonDocument{"property1": "value1", "property2": 2, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}
	m.SetContent(&d)

	// Act
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"application/json","content":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
}

func TestMessage_UnmarshalJSON_Plain(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world"}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", m.ID)
	assert.Nil(t, m.From)
	assert.Equal(t, &Node{Identity{"golang", "limeprotocol.org"}, "default"}, m.To)
	d, ok := m.Content.(*PlainDocument)
	assert.True(t, ok)
	assert.Equal(t, PlainDocument("Hello world"), *d)
}

func TestMessage_UnmarshalJSON_Json(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"application/json","content":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", m.ID)
	assert.Nil(t, m.From)
	assert.Equal(t, &Node{Identity{"golang", "limeprotocol.org"}, "default"}, m.To)
	d, ok := m.Content.(*JsonDocument)
	assert.True(t, ok)

	assert.Equal(t, JsonDocument{"property1": "value1", "property2": 2.0, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}, *d)
}
