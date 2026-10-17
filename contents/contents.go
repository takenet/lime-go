// Package contents provides standard LIME message content documents.
package contents

import (
	"encoding/json"
	"errors"

	"github.com/takenet/lime-go/document"
	"github.com/takenet/lime-go/protocol"
)

func RegisterContents() {
	for _, factory := range []func() document.Document{
		func() document.Document { return &ChatState{} },
		func() document.Document { return &Input{} },
		func() document.Document { return &Select{} },
		func() document.Document { return &DocumentSelect{} },
		func() document.Document { return &Location{} },
		func() document.Document { return &MediaLink{} },
		func() document.Document { return &WebLink{} },
		func() document.Document { return &DeletedContent{} },
		func() document.Document { return &External{} },
		func() document.Document { return &DocumentList{} },
		func() document.Document { return &DocumentTuple{} },
		func() document.Document { return &SensitiveContainer{} },
		func() document.Document { return &Redirect{} },
		func() document.Document { return &Reply{} },
		func() document.Document { return &Reaction{} },
		func() document.Document { return &CopyAndPaste{} },
		func() document.Document { return &SsmlText{} },
	} {
		document.RegisterDocumentFactory(factory)
	}
}

func mediaType(subtype string) document.MediaType {
	return document.MediaType{Type: document.MediaTypeApplication, Subtype: "vnd.lime." + subtype, Suffix: "json"}
}

type ChatStateEvent string

const (
	ChatStateStarting  ChatStateEvent = "starting"
	ChatStateComposing ChatStateEvent = "composing"
	ChatStatePaused    ChatStateEvent = "paused"
	ChatStateDeleting  ChatStateEvent = "deleting"
	ChatStateGone      ChatStateEvent = "gone"
)

type ChatState struct {
	State    ChatStateEvent `json:"state"`
	Interval *int           `json:"interval,omitempty"`
}

func MediaTypeChatState() document.MediaType     { return mediaType("chatstate") }
func (*ChatState) MediaType() document.MediaType { return MediaTypeChatState() }

type Input struct {
	Label      *document.DocumentContainer `json:"label,omitempty"`
	Validation *InputValidation            `json:"validation,omitempty"`
}

func MediaTypeInput() document.MediaType     { return mediaType("input") }
func (*Input) MediaType() document.MediaType { return MediaTypeInput() }

type InputValidationRule string

const (
	InputValidationText   InputValidationRule = "text"
	InputValidationNumber InputValidationRule = "number"
	InputValidationDate   InputValidationRule = "date"
	InputValidationRegex  InputValidationRule = "regex"
	InputValidationType   InputValidationRule = "type"
)

type InputValidation struct {
	Rule  InputValidationRule `json:"rule"`
	Regex string              `json:"regex,omitempty"`
	Type  *document.MediaType `json:"type,omitempty"`
	Error string              `json:"error,omitempty"`
}

type SelectScope string

const (
	SelectScopeTransient  SelectScope = "transient"
	SelectScopePersistent SelectScope = "persistent"
	SelectScopeImmediate  SelectScope = "immediate"
)

type Select struct {
	Scope   *SelectScope   `json:"scope,omitempty"`
	Text    string         `json:"text,omitempty"`
	Options []SelectOption `json:"options,omitempty"`
}

func MediaTypeSelect() document.MediaType     { return mediaType("select") }
func (*Select) MediaType() document.MediaType { return MediaTypeSelect() }

type SelectOption struct {
	Order *int
	Text  string
	Value document.Document
}

func (o SelectOption) MarshalJSON() ([]byte, error) {
	wire := struct {
		Order *int                `json:"order,omitempty"`
		Text  string              `json:"text,omitempty"`
		Type  *document.MediaType `json:"type,omitempty"`
		Value json.RawMessage     `json:"value,omitempty"`
	}{Order: o.Order, Text: o.Text}
	if o.Value != nil {
		mediaType := o.Value.MediaType()
		wire.Type = &mediaType
		value, err := json.Marshal(o.Value)
		if err != nil {
			return nil, err
		}
		wire.Value = value
	}
	return json.Marshal(wire)
}

func (o *SelectOption) UnmarshalJSON(data []byte) error {
	var wire struct {
		Order *int                `json:"order"`
		Text  string              `json:"text"`
		Type  *document.MediaType `json:"type"`
		Value json.RawMessage     `json:"value"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeTypedDocument(wire.Type, wire.Value)
	if err != nil {
		return err
	}
	o.Order, o.Text, o.Value = wire.Order, wire.Text, value
	return nil
}

type DocumentSelect struct {
	Scope   *SelectScope                `json:"scope,omitempty"`
	Header  *document.DocumentContainer `json:"header,omitempty"`
	Options []DocumentSelectOption      `json:"options,omitempty"`
}

func MediaTypeDocumentSelect() document.MediaType     { return mediaType("document-select") }
func (*DocumentSelect) MediaType() document.MediaType { return MediaTypeDocumentSelect() }

type DocumentSelectOption struct {
	Order *int                        `json:"order,omitempty"`
	Label *document.DocumentContainer `json:"label,omitempty"`
	Value *document.DocumentContainer `json:"value,omitempty"`
}

type Location struct {
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Altitude  *float64 `json:"altitude,omitempty"`
	Course    *int     `json:"course,omitempty"`
	Speed     *float64 `json:"speed,omitempty"`
	Accuracy  *float64 `json:"accuracy,omitempty"`
	Text      string   `json:"text,omitempty"`
}

func MediaTypeLocation() document.MediaType     { return mediaType("location") }
func (*Location) MediaType() document.MediaType { return MediaTypeLocation() }

type Link struct {
	URI                string              `json:"uri,omitempty"`
	PreviewURI         string              `json:"previewUri,omitempty"`
	PreviewType        *document.MediaType `json:"previewType,omitempty"`
	Title              string              `json:"title,omitempty"`
	Text               string              `json:"text,omitempty"`
	AuthorizationRealm string              `json:"authorizationRealm,omitempty"`
}

type MediaLink struct {
	Link
	Type        *document.MediaType `json:"type,omitempty"`
	Size        *int64              `json:"size,omitempty"`
	AspectRatio string              `json:"aspectRatio,omitempty"`
}

func MediaTypeMediaLink() document.MediaType     { return mediaType("media-link") }
func (*MediaLink) MediaType() document.MediaType { return MediaTypeMediaLink() }

type WebLinkTarget string

const (
	WebLinkTargetBlank       WebLinkTarget = "blank"
	WebLinkTargetSelf        WebLinkTarget = "self"
	WebLinkTargetSelfCompact WebLinkTarget = "selfCompact"
	WebLinkTargetSelfTall    WebLinkTarget = "selfTall"
)

type WebLink struct {
	Link
	Target *WebLinkTarget `json:"target,omitempty"`
}

func MediaTypeWebLink() document.MediaType     { return mediaType("web-link") }
func (*WebLink) MediaType() document.MediaType { return MediaTypeWebLink() }

type DeletedContent struct{}

func MediaTypeDeletedContent() document.MediaType     { return mediaType("deleted-content") }
func (*DeletedContent) MediaType() document.MediaType { return MediaTypeDeletedContent() }

type External struct {
	Content document.Document `json:"content,omitempty"`
}

func MediaTypeExternal() document.MediaType     { return mediaType("external") }
func (*External) MediaType() document.MediaType { return MediaTypeExternal() }

func (e External) MarshalJSON() ([]byte, error) {
	mediaType, value, err := encodeTypedDocument(e.Content)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Type    *document.MediaType `json:"type,omitempty"`
		Content json.RawMessage     `json:"content,omitempty"`
	}{mediaType, value})
}

func (e *External) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type    *document.MediaType `json:"type"`
		Content json.RawMessage     `json:"content"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	content, err := decodeTypedDocument(wire.Type, wire.Content)
	if err != nil {
		return err
	}
	e.Content = content
	return nil
}

type DocumentList struct {
	Header *document.DocumentContainer   `json:"header,omitempty"`
	Items  []*document.DocumentContainer `json:"items,omitempty"`
}

func MediaTypeDocumentList() document.MediaType     { return mediaType("list") }
func (*DocumentList) MediaType() document.MediaType { return MediaTypeDocumentList() }

type DocumentTuple struct {
	Item1 *document.DocumentContainer `json:"item1,omitempty"`
	Item2 *document.DocumentContainer `json:"item2,omitempty"`
	Item3 *document.DocumentContainer `json:"item3,omitempty"`
	Item4 *document.DocumentContainer `json:"item4,omitempty"`
	Item5 *document.DocumentContainer `json:"item5,omitempty"`
}

func MediaTypeDocumentTuple() document.MediaType     { return mediaType("tuple") }
func (*DocumentTuple) MediaType() document.MediaType { return MediaTypeDocumentTuple() }

type SensitiveContainer struct {
	Value document.Document `json:"value,omitempty"`
}

func MediaTypeSensitiveContainer() document.MediaType     { return mediaType("sensitive") }
func (*SensitiveContainer) MediaType() document.MediaType { return MediaTypeSensitiveContainer() }

func (s SensitiveContainer) MarshalJSON() ([]byte, error) {
	mediaType, value, err := encodeTypedDocument(s.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		Type  *document.MediaType `json:"type,omitempty"`
		Value json.RawMessage     `json:"value,omitempty"`
	}{mediaType, value})
}

func (s *SensitiveContainer) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type  *document.MediaType `json:"type"`
		Value json.RawMessage     `json:"value"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeTypedDocument(wire.Type, wire.Value)
	if err != nil {
		return err
	}
	s.Value = value
	return nil
}

type Redirect struct {
	Address protocol.Node               `json:"address"`
	Context *document.DocumentContainer `json:"context,omitempty"`
}

func MediaTypeRedirect() document.MediaType     { return mediaType("redirect") }
func (*Redirect) MediaType() document.MediaType { return MediaTypeRedirect() }

type MessageDirection string

const (
	MessageDirectionSent     MessageDirection = "sent"
	MessageDirectionReceived MessageDirection = "received"
)

type InReplyTo struct {
	ID        string
	Value     document.Document
	Direction *MessageDirection
}

func (r InReplyTo) MarshalJSON() ([]byte, error) {
	mediaType, value, err := encodeTypedDocument(r.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID        string              `json:"id,omitempty"`
		Type      *document.MediaType `json:"type,omitempty"`
		Value     json.RawMessage     `json:"value,omitempty"`
		Direction *MessageDirection   `json:"direction,omitempty"`
	}{r.ID, mediaType, value, r.Direction})
}

func (r *InReplyTo) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID        string              `json:"id"`
		Type      *document.MediaType `json:"type"`
		Value     json.RawMessage     `json:"value"`
		Direction *MessageDirection   `json:"direction"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeTypedDocument(wire.Type, wire.Value)
	if err != nil {
		return err
	}
	r.ID, r.Value, r.Direction = wire.ID, value, wire.Direction
	return nil
}

type Reply struct {
	Replied   *document.DocumentContainer `json:"replied,omitempty"`
	InReplyTo *InReplyTo                  `json:"inReplyTo,omitempty"`
}

func MediaTypeReply() document.MediaType     { return mediaType("reply") }
func (*Reply) MediaType() document.MediaType { return MediaTypeReply() }

type UnicodeSequence struct {
	Values []uint32 `json:"values,omitempty"`
}

func (s UnicodeSequence) String() string {
	var builder []rune
	for _, value := range s.Values {
		builder = append(builder, rune(value))
	}
	return string(builder)
}

type InReactionTo struct {
	ID        string
	Value     document.Document
	Direction *MessageDirection
}

func (r InReactionTo) MarshalJSON() ([]byte, error) {
	mediaType, value, err := encodeTypedDocument(r.Value)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID        string              `json:"id,omitempty"`
		Type      *document.MediaType `json:"type,omitempty"`
		Value     json.RawMessage     `json:"value,omitempty"`
		Direction *MessageDirection   `json:"direction,omitempty"`
	}{r.ID, mediaType, value, r.Direction})
}

func (r *InReactionTo) UnmarshalJSON(data []byte) error {
	var wire struct {
		ID        string              `json:"id"`
		Type      *document.MediaType `json:"type"`
		Value     json.RawMessage     `json:"value"`
		Direction *MessageDirection   `json:"direction"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeTypedDocument(wire.Type, wire.Value)
	if err != nil {
		return err
	}
	r.ID, r.Value, r.Direction = wire.ID, value, wire.Direction
	return nil
}

type Reaction struct {
	Emoji        *UnicodeSequence `json:"emoji,omitempty"`
	InReactionTo *InReactionTo    `json:"inReactionTo,omitempty"`
}

func MediaTypeReaction() document.MediaType     { return mediaType("reaction") }
func (*Reaction) MediaType() document.MediaType { return MediaTypeReaction() }

type Button struct {
	Text  string `json:"text,omitempty"`
	Value string `json:"value,omitempty"`
}

type CopyAndPaste struct {
	Header string  `json:"header,omitempty"`
	Body   string  `json:"body,omitempty"`
	Footer string  `json:"footer,omitempty"`
	Button *Button `json:"button,omitempty"`
}

func MediaTypeCopyAndPaste() document.MediaType     { return mediaType("copy-and-paste") }
func (*CopyAndPaste) MediaType() document.MediaType { return MediaTypeCopyAndPaste() }

type SsmlText struct {
	Text string `json:"text,omitempty"`
	Ssml string `json:"ssml,omitempty"`
}

func MediaTypeSsmlText() document.MediaType     { return mediaType("ssml-text") }
func (*SsmlText) MediaType() document.MediaType { return MediaTypeSsmlText() }

func encodeTypedDocument(value document.Document) (*document.MediaType, json.RawMessage, error) {
	if value == nil {
		return nil, nil, nil
	}
	mediaType := value.MediaType()
	encoded, err := json.Marshal(value)
	return &mediaType, encoded, err
}

func decodeTypedDocument(mediaType *document.MediaType, value json.RawMessage) (document.Document, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	if mediaType == nil {
		return nil, errors.New("document media type is required")
	}
	return document.UnmarshalDocument(&value, *mediaType)
}
