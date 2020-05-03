package lime

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
