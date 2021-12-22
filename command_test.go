package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommand_MarshalJSON_GetPingRequest(t *testing.T) {
	// Arrange
	c := createGetPingCommand()

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"get","uri":"/ping"}`, string(b))
}

func TestCommand_MarshalJSON_MergeDocumentContainerRequest(t *testing.T) {
	// Arrange
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.To = Node{}
	c.To.Name = "postmaster"
	c.To.Domain = "limeprotocol.org"
	c.Method = CommandMethodMerge
	u, _ := ParseLimeURI("/document/john.doe%40limeprotocol.org")
	c.URI = &u
	d := DocumentContainer{
		Type: MediaType{"application", "vnd.lime.account", "json"},
		Value: &JsonDocument{
			"name":    "John Doe",
			"address": "Main street",
			"city":    "Belo Horizonte",
			"extras": map[string]interface{}{
				"plan": "premium",
			},
		},
	}
	c.SetResource(&d)

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"merge","uri":"/document/john.doe%40limeprotocol.org","type":"application/vnd.lime.container+json","resource":{"type":"application/vnd.lime.account+json","value":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}}`, string(b))
}

func TestCommand_MarshalJSON_GetAccountResponse(t *testing.T) {
	// Arrange
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = "limeprotocol.org"
	c.From.Instance = "#server1"
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = "limeprotocol.org"
	c.To.Instance = "default"
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	a := JsonDocument{"name": "John Doe", "address": "Main street", "city": "Belo Horizonte", "extras": map[string]interface{}{"plan": "premium"}}
	c.Resource = &a
	m, _ := ParseMediaType("application/vnd.lime.account+json")
	c.Type = &m

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.account+json","resource":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}`, string(b))
}

func TestCommand_MarshalJSON_GetAccountCollectionResponse(t *testing.T) {
	// Arrange
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = "limeprotocol.org"
	c.From.Instance = "#server1"
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = "limeprotocol.org"
	c.To.Instance = "default"
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	collection := DocumentCollection{
		Total:    3,
		ItemType: MediaType{"application", "vnd.lime.account", "json"},
		Items: []Document{
			&JsonDocument{"name": "John Doe", "address": "Main street", "city": "Belo Horizonte", "extras": map[string]interface{}{"plan": "premium"}},
			&JsonDocument{"name": "Alice", "address": "Wonderland"},
			&JsonDocument{"name": "Bob", "city": "New York"},
		},
	}

	c.SetResource(&collection)

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.collection+json","resource":{"total":3,"itemType":"application/vnd.lime.account+json","items":[{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}},{"name":"Alice","address":"Wonderland"},{"name":"Bob","city":"New York"}]}}`, string(b))
}

func TestCommand_MarshalJSON_SetFailureResponse(t *testing.T) {
	// Arrange
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = "limeprotocol.org"
	c.From.Instance = "#server1"
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = "limeprotocol.org"
	c.To.Instance = "default"
	c.Method = CommandMethodSet
	c.Status = CommandStatusFailure
	c.Reason = &Reason{
		Code:        101,
		Description: "The resource was not found",
	}

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"set","status":"failure","reason":{"code":101,"description":"The resource was not found"}}`, string(b))
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
	assert.Zero(t, c.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	u, _ := ParseLimeURI("/ping")
	assert.Equal(t, u, *c.URI)
	assert.Zero(t, c.Status)
	assert.Nil(t, c.Resource)
}

func TestCommand_UnmarshalJSON_MergeDocumentContainerRequest(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"merge","uri":"/documentContainer/john.doe%40limeprotocol.org","type":"application/vnd.lime.container+json","resource":{"type":"application/vnd.lime.account+json","value":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}}`)
	var c Command

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", c.ID)
	assert.Zero(t, c.From)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, ""}, c.To)
	assert.Equal(t, CommandMethodMerge, c.Method)
	assert.NotNil(t, c.URI)
	u, _ := ParseLimeURI("/documentContainer/john.doe%40limeprotocol.org")
	assert.Equal(t, u, *c.URI)
	assert.Zero(t, c.Status)
	assert.NotNil(t, c.Type)
	assert.Equal(t, MediaType{"application", "vnd.lime.container", "json"}, *c.Type)
	assert.NotNil(t, c.Resource)
	dc, ok := c.Resource.(*DocumentContainer)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	documentContainer := *dc
	assert.Equal(t, MediaType{"application", "vnd.lime.account", "json"}, documentContainer.Type)
	d, ok := documentContainer.Value.(*JsonDocument)
	assert.True(t, ok)
	document := *d
	assert.Equal(t, "John Doe", document["name"])
	assert.Equal(t, "Main street", document["address"])
	assert.Equal(t, "Belo Horizonte", document["city"])
	assert.Equal(t, map[string]interface{}{"plan": "premium"}, document["extras"])
}

func TestCommand_UnmarshalJSON_GetAccountResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.account+json","resource":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}`)
	var c Command

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", c.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, c.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	assert.Equal(t, CommandStatusSuccess, c.Status)
	assert.NotNil(t, c.Type)
	assert.Equal(t, MediaType{"application", "vnd.lime.account", "json"}, *c.Type)
	assert.NotNil(t, c.Resource)
	d, ok := c.Resource.(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	document := *d
	assert.Equal(t, "John Doe", document["name"])
	assert.Equal(t, "Main street", document["address"])
	assert.Equal(t, "Belo Horizonte", document["city"])
	assert.Equal(t, map[string]interface{}{"plan": "premium"}, document["extras"])
}

func TestCommand_UnmarshalJSON_GetAccountCollectionResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.collection+json","resource":{"total":3,"itemType":"application/vnd.lime.account+json","items":[{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}},{"name":"Alice","address":"Wonderland"},{"name":"Bob","city":"New York"}]}}`)
	var c Command

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", c.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, c.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	assert.Equal(t, CommandStatusSuccess, c.Status)
	assert.NotNil(t, c.Type)
	assert.Equal(t, MediaType{"application", "vnd.lime.collection", "json"}, *c.Type)
	assert.NotNil(t, c.Resource)
	d, ok := c.Resource.(*DocumentCollection)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	collection := *d
	assert.Equal(t, 3, collection.Total)
	assert.Equal(t, MediaType{"application", "vnd.lime.account", "json"}, collection.ItemType)
	assert.Len(t, collection.Items, 3)
	a1, ok := collection.Items[0].(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	account1 := *a1
	assert.Equal(t, "John Doe", account1["name"])
	assert.Equal(t, "Main street", account1["address"])
	assert.Equal(t, "Belo Horizonte", account1["city"])
	assert.Equal(t, map[string]interface{}{"plan": "premium"}, account1["extras"])
	a2, ok := collection.Items[1].(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	account2 := *a2
	assert.Equal(t, "Alice", account2["name"])
	assert.Equal(t, "Wonderland", account2["address"])
	a3, ok := collection.Items[2].(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	account3 := *a3
	assert.Equal(t, "Bob", account3["name"])
	assert.Equal(t, "New York", account3["city"])
}

func TestCommand_UnmarshalJSON_SetFailureResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"set","status":"failure","reason":{"code":101,"description":"The resource was not found"}}`)
	var c Command

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, "4609d0a3-00eb-4e16-9d44-27d115c6eb31", c.ID)
	assert.Equal(t, Node{Identity{"postmaster", "limeprotocol.org"}, "#server1"}, c.From)
	assert.Equal(t, Node{Identity{"golang", "limeprotocol.org"}, "default"}, c.To)
	assert.Equal(t, CommandMethodSet, c.Method)
	assert.Equal(t, CommandStatusFailure, c.Status)
	assert.NotNil(t, c.Reason)
	assert.Equal(t, Reason{101, "The resource was not found"}, *c.Reason)
	assert.Zero(t, c.Type)
	assert.Nil(t, c.Resource)
}

func createGetPingCommand() *Command {
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.To = Node{}
	c.To.Name = "postmaster"
	c.To.Domain = "limeprotocol.org"
	c.Method = CommandMethodGet
	u, _ := ParseLimeURI("/ping")
	c.URI = &u

	return &c
}

func createResponseCommand() *Command {
	c := Command{}
	c.ID = "4609d0a3-00eb-4e16-9d44-27d115c6eb31"
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = "limeprotocol.org"
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	return &c
}
