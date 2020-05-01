package lime

type MediaType struct {
}

type Document interface {
	getMediaType() MediaType
}
