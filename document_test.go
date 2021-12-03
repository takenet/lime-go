package lime

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func createPlainDocument() PlainDocument {
	return PlainDocument("Hello world!")
}

func createJsonDocument() *JsonDocument {
	return &JsonDocument{"property1": "value1", "property2": 2.0, "property3": map[string]interface{}{"subproperty1": "subvalue1"}, "property4": false, "property5": 12.3}
}

func createTestJsonDocument() *testJsonDocument {
	return &testJsonDocument{
		Property1: "value1",
		Property2: 2,
		Property3: map[string]interface{}{"subproperty1": "subvalue1"},
		Property4: false,
		Property5: 12.3,
	}
}

type testJsonDocument struct {
	Property1 string                 `json:"property1"`
	Property2 int                    `json:"property2"`
	Property3 map[string]interface{} `json:"property3"`
	Property4 bool                   `json:"property4"`
	Property5 float32                `json:"property5"`
}

func mediaTypeTestJson() MediaType {
	return MediaType{
		Type:    "application",
		Subtype: "x-lime-test",
		Suffix:  "json",
	}
}

func (t *testJsonDocument) GetMediaType() MediaType {
	return mediaTypeTestJson()
}

func TestDocumentContainer_MarshalJSON_Plain(t *testing.T) {
	// Arrange
	d := createPlainDocument()
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
	d := createJsonDocument()
	c := NewDocumentContainer(d)

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"type":"application/json","value":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
}

func TestDocumentContainer_MarshalJSON_CustomJSON(t *testing.T) {
	// Arrange
	d := createTestJsonDocument()
	c := NewDocumentContainer(d)

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"type":"application/x-lime-test+json","value":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`, string(b))
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
	assert.True(t, ok)
	assert.Equal(t, createPlainDocument(), *actual)
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
	assert.True(t, ok)
	assert.Equal(t, *createJsonDocument(), *actual)
}

func TestDocumentContainer_UnmarshalJSON_CustomJSON(t *testing.T) {
	// Arrange
	j := []byte(`{"type":"application/x-lime-test+json","value":{"property1":"value1", "property2":2,"property3":{"subproperty1":"subvalue1"},"property4":false,"property5":12.3}}`)
	var d DocumentContainer
	RegisterDocumentFactory(func() Document {
		return &testJsonDocument{}
	})

	// Act
	err := json.Unmarshal(j, &d)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, mediaTypeTestJson(), d.Type)
	actual, ok := d.Value.(*testJsonDocument)
	assert.True(t, ok)
	assert.Equal(t, *createTestJsonDocument(), *actual)
}

func TestDocumentCollection_MarshalJSON_Plain(t *testing.T) {
	// Arrange
	items := make([]Document, 3)
	for i := 0; i < len(items); i++ {
		items[i] = PlainDocument(fmt.Sprintf("Hello world %v!", i+1))
	}
	c := NewDocumentCollection(items, MediaTypeTextPlain())

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"total":3,"itemType":"text/plain","items":["Hello world 1!","Hello world 2!","Hello world 3!"]}`, string(b))
}

func TestDocumentCollection_MarshalJSON_JSON(t *testing.T) {
	// Arrange
	items := make([]Document, 3)
	for i := 0; i < len(items); i++ {
		items[i] = &JsonDocument{"text": fmt.Sprintf("Hello world %v!", i+1)}
	}
	c := NewDocumentCollection(items, MediaTypeApplicationJson())

	// Act
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.JSONEq(t, `{"total":3,"itemType":"application/json","items":[{"text":"Hello world 1!"},{"text":"Hello world 2!"},{"text":"Hello world 3!"}]}`, string(b))
}

func TestDocumentCollection_UnmarshalJSON_Plain(t *testing.T) {
	// Arrange
	j := []byte(`{"total":3,"itemType":"text/plain","items":["Hello world 1!","Hello world 2!","Hello world 3!"]}`)
	var c DocumentCollection

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, 3, c.Total)
	assert.Equal(t, MediaTypeTextPlain(), c.ItemType)
	for i, d := range c.Items {
		actual, ok := d.(*PlainDocument)
		assert.True(t, ok)
		assert.Equal(t, PlainDocument(fmt.Sprintf("Hello world %v!", i+1)), *actual)
	}
}

func TestDocumentCollection_UnmarshalJSON_JSON(t *testing.T) {
	// Arrange
	j := []byte(`{"total":3,"itemType":"application/json","items":[{"text":"Hello world 1!"},{"text":"Hello world 2!"},{"text":"Hello world 3!"}]}`)
	var c DocumentCollection

	// Act
	err := json.Unmarshal(j, &c)
	if err != nil {
		t.Fatal(err)
	}

	// Assert
	assert.Equal(t, 3, c.Total)
	assert.Equal(t, MediaTypeApplicationJson(), c.ItemType)
	for i, d := range c.Items {
		actual, ok := d.(*JsonDocument)
		assert.True(t, ok)
		assert.Equal(t, JsonDocument{"text": fmt.Sprintf("Hello world %v!", i+1)}, *actual)
	}
}
