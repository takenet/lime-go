package lime

import (
	"encoding/json"
	"errors"
)

// Defines an entity with a media type.
type Document interface {
	// Gets the type of the media for the document.
	GetMediaType() MediaType
}

// Represents a generic JSON document.
type JsonDocument map[string]interface{}

func (d *JsonDocument) GetMediaType() MediaType {
	return mediaTypeApplicationJson
}

// Represents a plain document.
type PlainDocument string

func (d *PlainDocument) GetMediaType() MediaType {
	return mediaTypeTextPlain
}

// Represents a generic container for a document, providing a media type for the correct handling of its value by the nodes.
// This class can be used along with DocumentCollection to traffic different document types in a single message.
type DocumentContainer struct {
	// The media type of the contained document.
	Type MediaType
	// The contained document value.
	Value Document
}

func (d *DocumentContainer) GetMediaType() MediaType {
	return MediaType{MediaTypeApplication, "vnd.lime.container", "json"}
}

type DocumentContainerWrapper struct {
	Type  *MediaType       `json:"type"`
	Value *json.RawMessage `json:"value"`
}

func (d DocumentContainer) MarshalJSON() ([]byte, error) {
	dw, err := d.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(dw)
}

func (d *DocumentContainer) UnmarshalJSON(b []byte) error {
	dw := DocumentContainerWrapper{}
	err := json.Unmarshal(b, &dw)
	if err != nil {
		return err
	}

	documentContainer := DocumentContainer{}
	err = documentContainer.populate(&dw)
	if err != nil {
		return err
	}

	*d = documentContainer
	return nil
}

func (d *DocumentContainer) toWrapper() (DocumentContainerWrapper, error) {
	dw := DocumentContainerWrapper{
		Type: &d.Type,
	}

	b, err := json.Marshal(d.Value)
	if err != nil {
		return DocumentContainerWrapper{}, err
	}
	r := json.RawMessage(b)
	dw.Value = &r

	return dw, nil
}

func (d *DocumentContainer) populate(dw *DocumentContainerWrapper) error {
	// Create the document type instance and unmarshal the json to it
	if dw.Type == nil {
		return errors.New("document' type is required")
	}

	document, err := UnmarshalDocument(dw.Value, *dw.Type)
	if err != nil {
		return err
	}

	d.Type = *dw.Type
	d.Value = document
	return nil
}

func init() {
	RegisterDocumentFactory(func() Document {
		d := PlainDocument("")
		return &d
	})
	RegisterDocumentFactory(func() Document {
		return &JsonDocument{}
	})
	RegisterDocumentFactory(func() Document {
		return &DocumentContainer{}
	})
}
