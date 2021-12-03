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

// Document defines an entity with a media type.
type Document interface {
	// GetMediaType gets the type of the media for the document.
	GetMediaType() MediaType
}

// JsonDocument represents a generic JSON document.
type JsonDocument map[string]interface{}

func (d *JsonDocument) GetMediaType() MediaType {
	return MediaTypeApplicationJson()
}

// PlainDocument represents a plain document.
type PlainDocument string

func (d PlainDocument) GetMediaType() MediaType {
	return MediaTypeTextPlain()
}

// DocumentContainer represents a generic container for a document,
// providing a media type for the correct handling of its value by the nodes.
// This type can be used along with DocumentCollection to transport distinct
// document types in a single message.
type DocumentContainer struct {
	// The media type of the contained document.
	Type MediaType
	// The contained document value.
	Value Document
}

func NewDocumentContainer(d Document) *DocumentContainer {
	return &DocumentContainer{
		Type:  d.GetMediaType(),
		Value: d,
	}
}

func (d *DocumentContainer) GetMediaType() MediaType {
	return MediaType{
		MediaTypeApplication,
		"vnd.lime.container",
		"json",
	}
}

// rawDocumentContainer is a wrapper for custom marshalling
type rawDocumentContainer struct {
	Type  *MediaType       `json:"type"`
	Value *json.RawMessage `json:"value"`
}

func (d *DocumentContainer) MarshalJSON() ([]byte, error) {
	dw, err := d.raw()
	if err != nil {
		return nil, err
	}
	return json.Marshal(dw)
}

func (d *DocumentContainer) UnmarshalJSON(b []byte) error {
	raw := rawDocumentContainer{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	documentContainer := DocumentContainer{}
	err = documentContainer.populate(&raw)
	if err != nil {
		return err
	}

	*d = documentContainer
	return nil
}

func (d *DocumentContainer) raw() (*rawDocumentContainer, error) {
	raw := rawDocumentContainer{
		Type: &d.Type,
	}

	b, err := json.Marshal(d.Value)
	if err != nil {
		return &rawDocumentContainer{}, err
	}
	r := json.RawMessage(b)
	raw.Value = &r

	return &raw, nil
}

func (d *DocumentContainer) populate(raw *rawDocumentContainer) error {
	// Create the document type instance and unmarshal the json to it
	if raw.Type == nil {
		return errors.New("document type is required")
	}

	document, err := UnmarshalDocument(raw.Value, *raw.Type)
	if err != nil {
		return err
	}

	d.Type = *raw.Type
	d.Value = document
	return nil
}

// DocumentCollection represents a collection of documents.
type DocumentCollection struct {
	// The total of items in the collection.
	// This value refers to the original source collection,
	// without any applied filter that may exist in the
	// items on this instance.
	Total int
	// The media type of all items of the collection.
	ItemType MediaType
	// The collection items.
	Items []Document
}

func (d *DocumentCollection) GetMediaType() MediaType {
	return MediaType{MediaTypeApplication, "vnd.lime.collection", "json"}
}

func NewDocumentCollection(items []Document, t MediaType) *DocumentCollection {
	return &DocumentCollection{
		Total:    len(items),
		ItemType: t,
		Items:    items,
	}
}

// rawDocumentCollection is a wrapper for custom marshalling
type rawDocumentCollection struct {
	Total    int                `json:"total,omitempty"`
	ItemType *MediaType         `json:"itemType"`
	Items    []*json.RawMessage `json:"items"`
}

func (d *DocumentCollection) MarshalJSON() ([]byte, error) {
	raw, err := d.raw()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (d *DocumentCollection) UnmarshalJSON(b []byte) error {
	raw := rawDocumentCollection{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	documentCollection := DocumentCollection{}
	err = documentCollection.populate(&raw)
	if err != nil {
		return err
	}

	*d = documentCollection
	return nil
}

func (d *DocumentCollection) raw() (*rawDocumentCollection, error) {
	raw := rawDocumentCollection{
		ItemType: &d.ItemType,
		Total:    d.Total,
	}

	if d.Items != nil {
		raw.Items = make([]*json.RawMessage, len(d.Items))

		for i, v := range d.Items {
			b, err := json.Marshal(v)
			if err != nil {
				return &rawDocumentCollection{}, err
			}
			r := json.RawMessage(b)
			raw.Items[i] = &r
		}
	}

	return &raw, nil
}

func (d *DocumentCollection) populate(raw *rawDocumentCollection) error {
	// Create the document type instance and unmarshal the json to it
	if raw.ItemType == nil {
		return errors.New("document collection item type is required")
	}

	if raw.Items != nil {
		d.Items = make([]Document, len(raw.Items))

		for i, v := range raw.Items {
			document, err := UnmarshalDocument(v, *raw.ItemType)
			if err != nil {
				return err
			}

			d.Items[i] = document
		}
	}

	d.ItemType = *raw.ItemType
	d.Total = raw.Total

	return nil
}
