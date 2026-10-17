package protocol

import (
	"encoding/json"

	"github.com/takenet/lime-go/address"
	"github.com/takenet/lime-go/document"
)

type Identity = address.Identity
type Node = address.Node

type EnvelopeType string

const (
	EnvelopeTypeMessage      EnvelopeType = "message"
	EnvelopeTypeNotification EnvelopeType = "notification"
	EnvelopeTypeCommand      EnvelopeType = "command"
	EnvelopeTypeSession      EnvelopeType = "session"
)

func ParseIdentity(value string) Identity { return address.ParseIdentity(value) }
func ParseNode(value string) Node         { return address.ParseNode(value) }

type Document = document.Document
type JsonDocument = document.JsonDocument
type TextDocument = document.TextDocument
type RawDocument = document.RawDocument
type DocumentContainer = document.DocumentContainer
type DocumentCollection = document.DocumentCollection
type MediaType = document.MediaType
type Ping = document.Ping

const (
	MediaTypeText        = document.MediaTypeText
	MediaTypeApplication = document.MediaTypeApplication
	MediaTypeImage       = document.MediaTypeImage
	MediaTypeAudio       = document.MediaTypeAudio
	MediaTypeVideo       = document.MediaTypeVideo
)

func NewDocumentContainer(value Document) *DocumentContainer {
	return document.NewDocumentContainer(value)
}

func NewDocumentCollection(items []Document, mediaType MediaType) *DocumentCollection {
	return document.NewDocumentCollection(items, mediaType)
}

func MediaTypePing() MediaType                        { return document.MediaTypePing() }
func ParseMediaType(value string) (MediaType, error)  { return document.ParseMediaType(value) }
func MediaTypeTextPlain() MediaType                   { return document.MediaTypeTextPlain() }
func MediaTypeApplicationJson() MediaType             { return document.MediaTypeApplicationJson() }
func RegisterDocumentFactory(factory func() Document) { document.RegisterDocumentFactory(factory) }
func GetDocumentFactory(mediaType MediaType) (func() Document, error) {
	return document.GetDocumentFactory(mediaType)
}
func UnmarshalDocument(raw *json.RawMessage, mediaType MediaType) (Document, error) {
	return document.UnmarshalDocument(raw, mediaType)
}
