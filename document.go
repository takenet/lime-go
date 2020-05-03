package lime

// Defines an entity with a media type.
type Document interface {
	// Gets the type of the media for the document.
	GetMediaType() MediaType
}

type JsonDocument map[string]interface{}

func (d *JsonDocument) GetMediaType() MediaType {
	return mediaTypeApplicationJson
}

type PlainDocument string

func (d *PlainDocument) GetMediaType() MediaType {
	return mediaTypeTextPlain
}
