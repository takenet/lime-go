package lime

import (
	"encoding/json"
	"errors"
)

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
	RegisterDocumentFactory(func() Document {
		return &DocumentCollection{}
	})
}

// Document Defines an entity with a media type.
type Document interface {
	// GetMediaType Gets the type of the media for the document.
	GetMediaType() MediaType
}

// JsonDocument Represents a generic JSON document.
type JsonDocument map[string]interface{}

func (d *JsonDocument) GetMediaType() MediaType {
	return mediaTypeApplicationJson
}

// PlainDocument Represents a plain document.
type PlainDocument string

func (d *PlainDocument) GetMediaType() MediaType {
	return mediaTypeTextPlain
}

// DocumentContainer Represents a generic container for a document, providing a media type for the correct handling of its value by the nodes.
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

// DocumentContainerWrapper Wrapper for custom marshalling
type DocumentContainerWrapper struct {
	Type  *MediaType       `json:"type"`
	Value *json.RawMessage `json:"value"`
}

func (d DocumentContainer) MarshalJSON() ([]byte, error) {
	dw, err := d.ToRawEnvelope()
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

func (d *DocumentContainer) ToRawEnvelope() (DocumentContainerWrapper, error) {
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
		return errors.New("document type is required")
	}

	document, err := UnmarshalDocument(dw.Value, *dw.Type)
	if err != nil {
		return err
	}

	d.Type = *dw.Type
	d.Value = document
	return nil
}

// DocumentCollection Represents a collection of documents.
type DocumentCollection struct {
	// The total of items in the collection.
	// This value refers to the original source collection, without any applied filter that may exist in the items on this instance.
	Total int
	// The media type of all items of the collection
	ItemType MediaType
	// The collection items.
	Items []Document
}

func (d *DocumentCollection) GetMediaType() MediaType {
	return MediaType{MediaTypeApplication, "vnd.lime.collection", "json"}
}

// DocumentCollectionWrapper Wrapper for custom marshalling
type DocumentCollectionWrapper struct {
	Total    int                `json:"total,omitempty"`
	ItemType *MediaType         `json:"itemType"`
	Items    []*json.RawMessage `json:"items"`
}

func (d DocumentCollection) MarshalJSON() ([]byte, error) {
	dw, err := d.ToRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(dw)
}

func (d *DocumentCollection) UnmarshalJSON(b []byte) error {
	dw := DocumentCollectionWrapper{}
	err := json.Unmarshal(b, &dw)
	if err != nil {
		return err
	}

	documentCollection := DocumentCollection{}
	err = documentCollection.populate(&dw)
	if err != nil {
		return err
	}

	*d = documentCollection
	return nil
}

func (d *DocumentCollection) ToRawEnvelope() (DocumentCollectionWrapper, error) {
	dw := DocumentCollectionWrapper{
		ItemType: &d.ItemType,
		Total:    d.Total,
	}

	if d.Items != nil {
		dw.Items = make([]*json.RawMessage, len(d.Items))

		for i, v := range d.Items {
			b, err := json.Marshal(v)
			if err != nil {
				return DocumentCollectionWrapper{}, err
			}
			r := json.RawMessage(b)
			dw.Items[i] = &r
		}
	}

	return dw, nil
}

func (d *DocumentCollection) populate(dw *DocumentCollectionWrapper) error {
	// Create the document type instance and unmarshal the json to it
	if dw.ItemType == nil {
		return errors.New("document collection item type is required")
	}

	if dw.Items != nil {
		d.Items = make([]Document, len(dw.Items))

		for i, v := range dw.Items {
			document, err := UnmarshalDocument(v, *dw.ItemType)
			if err != nil {
				return err
			}

			d.Items[i] = document
		}
	}

	d.ItemType = *dw.ItemType
	d.Total = dw.Total

	return nil
}
