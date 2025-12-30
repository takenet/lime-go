package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createMessage() *Message {
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	var d TextDocument = testHelloWorld
	m.SetContent(&d)
	return &m
}

func TestMessageMarshalJSONTextPlain(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	var d TextDocument = testHelloWorld
	m.SetContent(&d)

	// Act
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world"}`, string(b))
}

func TestMessageMarshalJSONMetadata(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	var d TextDocument = testHelloWorld
	m.SetContent(&d)
	m.Metadata = make(map[string]string)
	m.Metadata["property1"] = "value1"
	m.Metadata["property2"] = "value2"

	// Act
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world","metadata":{"property1":"value1","property2":"value2"}}`, string(b))
}

func TestMessageMarshalJSONTextUnknownPlain(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	var d TextDocument = testHelloWorld
	m.Content = d
	m.Type = MediaType{"text", "unknown", ""}

	// Act
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/unknown","content":"Hello world"}`, string(b))
}

func TestMessageMarshalJSONApplicationJson(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	d := JsonDocument{"property1": "value1", "property2": 2, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}
	m.SetContent(&d)

	// Act
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"application/json","content":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
}

func TestMessageMarshalJSONApplicationUnknownJson(t *testing.T) {
	// Arrange
	m := Message{}
	m.ID = testCommandID
	m.To = Node{}
	m.To.Name = "golang"
	m.To.Domain = testDomain
	m.To.Instance = "default"
	d := JsonDocument{"property1": "value1", "property2": 2, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}
	m.SetContent(&d)
	m.Type = MediaType{"application", "x-unknown", "json"}

	// Act
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"application/x-unknown+json","content":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
}

func TestMessageUnmarshalJSONTextPlain(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world"}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, m.ID)
	assert.Zero(t, m.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, m.To)
	assert.Equal(t, mediaTypeTextPlain, m.Type)
	d, ok := m.Content.(*TextDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, TextDocument(testHelloWorld), *d)
}

func TestMessageUnmarshalJSONMetadata(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","content":"Hello world","metadata":{"property1":"value1","property2":"value2"}}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, m.ID)
	assert.Zero(t, m.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, m.To)
	assert.Equal(t, mediaTypeTextPlain, m.Type)
	d, ok := m.Content.(*TextDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, TextDocument(testHelloWorld), *d)
	assert.Contains(t, m.Metadata, "property1")
	assert.Equal(t, "value1", m.Metadata["property1"])
	assert.Contains(t, m.Metadata, "property2")
	assert.Equal(t, "value2", m.Metadata["property2"])
}

func TestMessageUnmarshalJSONTextUnknownPlain(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/unknown","content":"Hello world"}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, m.ID)
	assert.Zero(t, m.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, m.To)
	assert.Equal(t, MediaType{"text", "unknown", ""}, m.Type)
	d, ok := m.Content.(*TextDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, TextDocument(testHelloWorld), *d)
}

func TestMessageUnmarshalJSONApplicationUnknownJson(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"application/x-unknown+json","content":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`)
	var m Message

	// Act
	err := json.Unmarshal(j, &m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, m.ID)
	assert.Zero(t, m.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, m.To)
	assert.Equal(t, MediaType{"application", "x-unknown", "json"}, m.Type)
	d, ok := m.Content.(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, JsonDocument{"property1": "value1", "property2": 2.0, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}, *d)
}

func TestMessageNotification(t *testing.T) {
	// Arrange
	msg := createMessage()
	msg.From = Node{Identity: Identity{Name: "sender", Domain: "example.com"}}

	// Act
	notification := msg.Notification(NotificationEventReceived)

	// Assert
	assert.NotNil(t, notification)
	assert.Equal(t, msg.ID, notification.ID)
	assert.Equal(t, msg.To, notification.From)
	assert.Equal(t, msg.From, notification.To)
	assert.Equal(t, NotificationEventReceived, notification.Event)
	assert.Nil(t, notification.Reason)
}

func TestMessageFailedNotification(t *testing.T) {
	// Arrange
	msg := createMessage()
	msg.From = Node{Identity: Identity{Name: "sender", Domain: "example.com"}}
	reason := &Reason{
		Code:        500,
		Description: "Internal error",
	}

	// Act
	notification := msg.FailedNotification(reason)

	// Assert
	assert.NotNil(t, notification)
	assert.Equal(t, msg.ID, notification.ID)
	assert.Equal(t, msg.To, notification.From)
	assert.Equal(t, msg.From, notification.To)
	assert.Equal(t, NotificationEventFailed, notification.Event)
	assert.Equal(t, reason, notification.Reason)
}
