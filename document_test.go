package lime

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocumentContainer_MarshalJSON_Plain(t *testing.T) {
	// Arrange
	d := PlainDocument("Hello world!")
	c := NewDocumentContainer(d)

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"type":"text/plain","value":"Hello world!"}`, string(b))
}

func TestDocumentContainer_MarshalJSON_JSON(t *testing.T) {
	// Arrange
	d := JsonDocument{"property1": "value1", "property2": 2, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}
	c := NewDocumentContainer(&d)

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"type":"application/json","value":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
}

func TestDocumentContainer_UnmarshalJSON_Plain(t *testing.T) {
	// Arrange
	j := []byte(`{"type":"text/plain","value":"Hello world!"}`)
	var d DocumentContainer

	// Act
	err := json.Unmarshal(j, &d)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, MediaTypeTextPlain(), d.Type)
	actual, ok := d.Value.(*PlainDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, PlainDocument("Hello world!"), *actual)
}

func TestDocumentContainer_UnmarshalJSON_JSON(t *testing.T) {
	// Arrange
	j := []byte(`{"type":"application/json","value":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`)
	var d DocumentContainer

	// Act
	err := json.Unmarshal(j, &d)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, MediaTypeApplicationJson(), d.Type)
	actual, ok := d.Value.(*JsonDocument)
	if !assert.True(t, ok) {
		t.Fatal()
	}
	assert.Equal(t, JsonDocument{"property1": "value1", "property2": 2.0, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}, *actual)
}
