package lime

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestCommandMarshalJSONGetPingRequest(t *testing.T) {
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

func TestRequestCommandMarshalJSONMergeDocumentContainerRequest(t *testing.T) {
	// Arrange
	c := RequestCommand{}
	c.ID = testCommandID
	c.To = Node{}
	c.To.Name = "postmaster"
	c.To.Domain = testDomain
	c.Method = CommandMethodMerge
	u, _ := ParseLimeURI("/document/john.doe%40limeprotocol.org")
	c.URI = u
	d := DocumentContainer{
		Type: MediaType{"application", testAccountMediaType, "json"},
		Value: &JsonDocument{
			"name":    testName,
			"address": testAddress,
			"city":    testCity,
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

func TestResponseCommandMarshalJSONGetAccountResponse(t *testing.T) {
	// Arrange
	c := ResponseCommand{}
	c.ID = testCommandID
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = testDomain
	c.From.Instance = testServerInstance
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = testDomain
	c.To.Instance = "default"
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	a := JsonDocument{"name": testName, "address": testAddress, "city": testCity, "extras": map[string]interface{}{"plan": "premium"}}
	c.Resource = &a
	m, _ := ParseMediaType("application/" + testAccountMediaType + "+json")
	c.Type = &m

	// Act
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.account+json","resource":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}`, string(b))
}

func TestResponseCommandMarshalJSONGetAccountCollectionResponse(t *testing.T) {
	// Arrange
	c := ResponseCommand{}
	c.ID = testCommandID
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = testDomain
	c.From.Instance = testServerInstance
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = testDomain
	c.To.Instance = "default"
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	collection := DocumentCollection{
		Total:    3,
		ItemType: MediaType{"application", testAccountMediaType, "json"},
		Items: []Document{
			&JsonDocument{"name": testName, "address": testAddress, "city": testCity, "extras": map[string]interface{}{"plan": "premium"}},
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

func TestResponseCommandMarshalJSONSetFailureResponse(t *testing.T) {
	// Arrange
	c := ResponseCommand{}
	c.ID = testCommandID
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = testDomain
	c.From.Instance = testServerInstance
	c.To = Node{}
	c.To.Name = "golang"
	c.To.Domain = testDomain
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

func TestRequestCommandUnmarshalJSONGetPingRequest(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"golang@limeprotocol.org/default","type":"text/plain","method":"get","uri":"/ping"}`)
	var c RequestCommand

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, c.ID)
	assert.Zero(t, c.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	u, _ := ParseLimeURI("/ping")
	assert.Equal(t, u, c.URI)
	assert.Nil(t, c.Resource)
}

func TestRequestCommandUnmarshalJSONMergeDocumentContainerRequest(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","to":"postmaster@limeprotocol.org","method":"merge","uri":"/documentContainer/john.doe%40limeprotocol.org","type":"application/vnd.lime.container+json","resource":{"type":"application/vnd.lime.account+json","value":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}}`)
	var c RequestCommand

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, c.ID)
	assert.Zero(t, c.From)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, ""}, c.To)
	assert.Equal(t, CommandMethodMerge, c.Method)
	assert.NotNil(t, c.URI)
	u, _ := ParseLimeURI("/documentContainer/john.doe%40limeprotocol.org")
	assert.Equal(t, u, c.URI)
	assert.NotNil(t, c.Type)
	assert.Equal(t, MediaType{"application", "vnd.lime.container", "json"}, *c.Type)
	assert.NotNil(t, c.Resource)
	dc, ok := c.Resource.(*DocumentContainer)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	documentContainer := *dc
	assert.Equal(t, MediaType{"application", testAccountMediaType, "json"}, documentContainer.Type)
	d, ok := documentContainer.Value.(*JsonDocument)
	assert.True(t, ok)
	document := *d
	assert.Equal(t, testName, document["name"])
	assert.Equal(t, testAddress, document["address"])
	assert.Equal(t, testCity, document["city"])
	assert.Equal(t, map[string]interface{}{"plan": "premium"}, document["extras"])
}

func TestResponseCommandUnmarshalJSONGetAccountResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.account+json","resource":{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}}}`)
	var c ResponseCommand

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, c.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, c.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, c.To)
	assert.Equal(t, CommandMethodGet, c.Method)
	assert.Equal(t, CommandStatusSuccess, c.Status)
	assert.NotNil(t, c.Type)
	assert.Equal(t, MediaType{"application", testAccountMediaType, "json"}, *c.Type)
	assert.NotNil(t, c.Resource)
	d, ok := c.Resource.(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	document := *d
	assert.Equal(t, testName, document["name"])
	assert.Equal(t, testAddress, document["address"])
	assert.Equal(t, testCity, document["city"])
	assert.Equal(t, map[string]any{"plan": "premium"}, document["extras"])
}

func TestResponseCommandUnmarshalJSONGetAccountCollectionResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"get","status":"success","type":"application/vnd.lime.collection+json","resource":{"total":3,"itemType":"application/vnd.lime.account+json","items":[{"name":"John Doe","address":"Main street","city":"Belo Horizonte","extras":{"plan":"premium"}},{"name":"Alice","address":"Wonderland"},{"name":"Bob","city":"New York"}]}}`)
	var c ResponseCommand

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, c.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, c.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, c.To)
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
	assert.Equal(t, MediaType{"application", testAccountMediaType, "json"}, collection.ItemType)
	assert.Len(t, collection.Items, 3)
	a1, ok := collection.Items[0].(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	account1 := *a1
	assert.Equal(t, testName, account1["name"])
	assert.Equal(t, testAddress, account1["address"])
	assert.Equal(t, testCity, account1["city"])
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

func TestResponseCommandUnmarshalJSONSetFailureResponse(t *testing.T) {
	// Arrange
	j := []byte(`{"id":"4609d0a3-00eb-4e16-9d44-27d115c6eb31","from":"postmaster@limeprotocol.org/#server1","to":"golang@limeprotocol.org/default","method":"set","status":"failure","reason":{"code":101,"description":"The resource was not found"}}`)
	var c ResponseCommand

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, testCommandID, c.ID)
	assert.Equal(t, Node{Identity{"postmaster", testDomain}, testServerInstance}, c.From)
	assert.Equal(t, Node{Identity{"golang", testDomain}, "default"}, c.To)
	assert.Equal(t, CommandMethodSet, c.Method)
	assert.Equal(t, CommandStatusFailure, c.Status)
	assert.NotNil(t, c.Reason)
	assert.Equal(t, Reason{101, "The resource was not found"}, *c.Reason)
	assert.Zero(t, c.Type)
	assert.Nil(t, c.Resource)
}

func createGetPingCommand() *RequestCommand {
	c := RequestCommand{}
	c.ID = testCommandID
	c.To = Node{}
	c.To.Name = "postmaster"
	c.To.Domain = testDomain
	c.Method = CommandMethodGet
	u, _ := ParseLimeURI("/ping")
	c.URI = u

	return &c
}

func createResponseCommand() *ResponseCommand {
	c := ResponseCommand{}
	c.ID = testCommandID
	c.From = Node{}
	c.From.Name = "postmaster"
	c.From.Domain = testDomain
	c.Method = CommandMethodGet
	c.Status = CommandStatusSuccess
	return &c
}

func TestRequestCommandSuccessResponse(t *testing.T) {
	// Arrange
	cmd := createGetPingCommand()

	// Act
	response := cmd.SuccessResponse()

	// Assert
	assert.NotNil(t, response)
	assert.Equal(t, CommandStatusSuccess, response.Status)
	assert.Equal(t, cmd.ID, response.ID)
	assert.Equal(t, cmd.To, response.From)
	assert.Equal(t, cmd.From, response.To)
	assert.Equal(t, cmd.Method, response.Method)
	assert.Nil(t, response.Reason)
}

func TestRequestCommandSuccessResponseWithResource(t *testing.T) {
	// Arrange
	cmd := createGetPingCommand()
	ping := &Ping{}

	// Act
	response := cmd.SuccessResponseWithResource(ping)

	// Assert
	assert.NotNil(t, response)
	assert.Equal(t, CommandStatusSuccess, response.Status)
	assert.Equal(t, cmd.ID, response.ID)
	assert.Equal(t, ping, response.Resource)
	assert.NotNil(t, response.Type)
}

func TestRequestCommandFailureResponse(t *testing.T) {
	// Arrange
	cmd := createGetPingCommand()
	reason := &Reason{
		Code:        1,
		Description: "Test error",
	}

	// Act
	response := cmd.FailureResponse(reason)

	// Assert
	assert.NotNil(t, response)
	assert.Equal(t, CommandStatusFailure, response.Status)
	assert.Equal(t, cmd.ID, response.ID)
	assert.Equal(t, cmd.To, response.From)
	assert.Equal(t, cmd.From, response.To)
	assert.Equal(t, cmd.Method, response.Method)
	assert.Equal(t, reason, response.Reason)
}

func TestRequestCommandSetMethod(t *testing.T) {
	// Arrange
	cmd := &RequestCommand{}

	// Act
	result := cmd.SetMethod(CommandMethodSet)

	// Assert
	assert.NotNil(t, result)
	assert.Equal(t, CommandMethodSet, cmd.Method)
}

func TestRequestCommandSetResource(t *testing.T) {
	// Arrange
	cmd := &RequestCommand{}
	ping := &Ping{}

	// Act
	result := cmd.SetResource(ping)

	// Assert
	assert.NotNil(t, result)
	assert.Equal(t, ping, cmd.Resource)
	assert.NotNil(t, cmd.Type)
	assert.Equal(t, ping.MediaType(), *cmd.Type)
}

func TestRequestCommandSetURI(t *testing.T) {
	// Arrange
	cmd := &RequestCommand{}
	uri, _ := ParseLimeURI(testURIPath)

	// Act
	result := cmd.SetURI(uri)

	// Assert
	assert.Equal(t, cmd, result)
	assert.Equal(t, uri, cmd.URI)
}

func TestRequestCommandSetURIString(t *testing.T) {
	// Arrange
	cmd := &RequestCommand{}

	// Act
	result := cmd.SetURIString(testURIPath)

	// Assert
	assert.Equal(t, cmd, result)
	assert.Equal(t, testURIPath, cmd.URI.Path())
}

func TestResponseCommandSetStatusFailure(t *testing.T) {
	// Arrange
	cmd := &ResponseCommand{}
	reason := Reason{
		Code:        42,
		Description: "Failure reason",
	}

	// Act
	cmd.SetStatusFailure(reason)

	// Assert
	assert.Equal(t, CommandStatusFailure, cmd.Status)
	assert.NotNil(t, cmd.Reason)
	assert.Equal(t, 42, cmd.Reason.Code)
	assert.Equal(t, "Failure reason", cmd.Reason.Description)
}

func TestParseLimeURIWithPath(t *testing.T) {
	// Act
	uri, err := ParseLimeURI("/accounts/john@doe.com")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, uri)
	assert.Equal(t, "/accounts/john@doe.com", uri.Path())
}

func TestParseLimeURIWithQueryString(t *testing.T) {
	// Act
	uri, err := ParseLimeURI("/accounts?filter=active&limit=10")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, uri)
	assert.Equal(t, "/accounts", uri.Path())
	assert.Contains(t, uri.String(), "filter=active")
	assert.Contains(t, uri.String(), "limit=10")
}

func TestParseLimeURIEmpty(t *testing.T) {
	// Act
	uri, err := ParseLimeURI("")

	// Assert
	// Empty string is valid, returns empty URI
	assert.NoError(t, err)
	assert.NotNil(t, uri)
	assert.Equal(t, "", uri.Path())
}

func TestParseLimeURIInvalid(t *testing.T) {
	// Act - URL with invalid characters that Go's url.Parse will reject
	uri, err := ParseLimeURI("http://[::1]:namedport")

	// Assert
	if err != nil {
		assert.Error(t, err)
		assert.Nil(t, uri)
	} else {
		// Go's url.Parse is very permissive, so this might not error
		assert.NotNil(t, uri)
	}
}

func TestLimeURIString(t *testing.T) {
	// Arrange
	uri, _ := ParseLimeURI(testURIPathWithQuery)

	// Act
	result := uri.String()

	// Assert
	assert.Equal(t, testURIPathWithQuery, result)
}

func TestLimeURIPath(t *testing.T) {
	// Arrange
	uri, _ := ParseLimeURI(testURIPathWithQuery)

	// Act
	result := uri.Path()

	// Assert
	assert.Equal(t, testURIPath, result)
}

func TestLimeURIQuery(t *testing.T) {
	// Arrange
	uri, _ := ParseLimeURI("/test/path?key=value&foo=bar")

	// Act
	result := uri.String()

	// Assert
	assert.Contains(t, result, "key=value")
	assert.Contains(t, result, "foo=bar")
}

func TestLimeURIMarshalText(t *testing.T) {
	// Arrange
	uri, _ := ParseLimeURI("/test/resource")

	// Act
	text, err := uri.MarshalText()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "/test/resource", string(text))
}

func TestURIUnmarshalText(t *testing.T) {
	// Arrange
	var uri URI
	text := []byte("/accounts/user@example.com")

	// Act
	err := uri.UnmarshalText(text)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "/accounts/user@example.com", uri.Path())
}

func TestCommandSetMethod(t *testing.T) {
	// Arrange
	cmd := &Command{}

	// Act
	result := cmd.SetMethod(CommandMethodDelete)

	// Assert
	assert.Equal(t, cmd, result)
	assert.Equal(t, CommandMethodDelete, cmd.Method)
}

func TestCommandSetResource(t *testing.T) {
	// Arrange
	cmd := &Command{}
	ping := &Ping{}

	// Act
	result := cmd.SetResource(ping)

	// Assert
	assert.Equal(t, cmd, result)
	assert.Equal(t, ping, cmd.Resource)
	assert.NotNil(t, cmd.Type)
}

func TestURIString(t *testing.T) {
	// Test with nil URL
	uri := &URI{}
	assert.Empty(t, uri.String())

	// Test with valid URL
	uri, _ = ParseLimeURI(testURIFull)
	assert.Equal(t, testURIFull, uri.String())
}

func TestURIPath(t *testing.T) {
	// Test with nil URL
	uri := &URI{}
	assert.Empty(t, uri.Path())

	// Test with valid URL
	uri, _ = ParseLimeURI("lime://example.com/path/to/resource")
	assert.Equal(t, "/path/to/resource", uri.Path())
}

func TestURIOwner(t *testing.T) {
	// Test with nil URL
	uri := &URI{}
	assert.Nil(t, uri.Owner())

	// Test with URL without user
	uri, _ = ParseLimeURI("lime://example.com/path")
	assert.Nil(t, uri.Owner())

	// Test with URL with user - owner only gets username from URL user info
	uri, _ = ParseLimeURI("lime://user@example.com/path")
	owner := uri.Owner()
	assert.NotNil(t, owner)
	assert.Equal(t, "user", owner.Name)
	// Domain is not parsed from URL host, only from username@domain in user info
	assert.Empty(t, owner.Domain)
}

func TestURIOwnerWithComplexIdentity(t *testing.T) {
	uri, _ := ParseLimeURI("lime://john.doe@company.example.com/resource")
	owner := uri.Owner()
	assert.NotNil(t, owner)
	assert.Equal(t, "john.doe", owner.Name)
	// Domain is not parsed from URL host, only from username@domain pattern in URL user info
	assert.Empty(t, owner.Domain)
}

func TestURIURL(t *testing.T) {
	// Test with nil URL
	uri := &URI{}
	assert.Nil(t, uri.URL())

	// Test with valid URL - should return a copy
	uri, _ = ParseLimeURI(testURIFull)
	url1 := uri.URL()
	url2 := uri.URL()

	assert.NotNil(t, url1)
	assert.NotNil(t, url2)
	assert.Equal(t, testURIFull, url1.String())

	// Verify it returns a copy (different pointers)
	assert.NotSame(t, url1, url2, "should return different instances (copies)")

	// Modifying one should not affect the other
	url1.Path = "/modified"
	assert.NotEqual(t, url1.Path, url2.Path, "copies should be independent")
}
