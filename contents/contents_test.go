package contents

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/takenet/lime-go/document"
)

func TestRegisterContents(t *testing.T) {
	RegisterContents()
	for _, test := range []struct {
		mediaType document.MediaType
		want      document.Document
	}{
		{MediaTypeChatState(), &ChatState{}},
		{MediaTypeInput(), &Input{}},
		{MediaTypeSelect(), &Select{}},
		{MediaTypeDocumentSelect(), &DocumentSelect{}},
		{MediaTypeLocation(), &Location{}},
		{MediaTypeMediaLink(), &MediaLink{}},
		{MediaTypeWebLink(), &WebLink{}},
		{MediaTypeDeletedContent(), &DeletedContent{}},
		{MediaTypeExternal(), &External{}},
		{MediaTypeDocumentList(), &DocumentList{}},
		{MediaTypeDocumentTuple(), &DocumentTuple{}},
		{MediaTypeSensitiveContainer(), &SensitiveContainer{}},
		{MediaTypeRedirect(), &Redirect{}},
		{MediaTypeReply(), &Reply{}},
		{MediaTypeReaction(), &Reaction{}},
		{MediaTypeCopyAndPaste(), &CopyAndPaste{}},
		{MediaTypeSsmlText(), &SsmlText{}},
	} {
		factory, err := document.GetDocumentFactory(test.mediaType)
		require.NoError(t, err)
		assert.IsType(t, test.want, factory())
	}
}

func TestSelectOptionRoundTripUsesDocumentMediaType(t *testing.T) {
	RegisterContents()
	option := SelectOption{
		Order: pointer(1),
		Text:  "Continue",
		Value: document.TextDocument("continue"),
	}
	encoded, err := json.Marshal(&Select{Text: "Choose", Options: []SelectOption{option}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"text":"Choose","options":[{"order":1,"text":"Continue","type":"text/plain","value":"continue"}]}`, string(encoded))

	decoded, err := document.UnmarshalDocument(rawJSON(encoded), MediaTypeSelect())
	require.NoError(t, err)
	selectDocument, ok := decoded.(*Select)
	require.True(t, ok)
	require.Len(t, selectDocument.Options, 1)
	text, ok := selectDocument.Options[0].Value.(*document.TextDocument)
	require.True(t, ok)
	assert.Equal(t, "continue", string(*text))
}

func TestDocumentSelectPreservesNestedDocuments(t *testing.T) {
	RegisterContents()
	content := &DocumentSelect{
		Header: document.NewDocumentContainer(document.TextDocument("Pick one")),
		Options: []DocumentSelectOption{{
			Order: pointer(1),
			Label: document.NewDocumentContainer(document.TextDocument("Yes")),
			Value: document.NewDocumentContainer(document.TextDocument("yes")),
		}},
	}
	encoded, err := json.Marshal(content)
	require.NoError(t, err)
	assert.JSONEq(t, `{"header":{"type":"text/plain","value":"Pick one"},"options":[{"order":1,"label":{"type":"text/plain","value":"Yes"},"value":{"type":"text/plain","value":"yes"}}]}`, string(encoded))

	decoded, err := document.UnmarshalDocument(rawJSON(encoded), MediaTypeDocumentSelect())
	require.NoError(t, err)
	got := decoded.(*DocumentSelect)
	header, ok := got.Header.Value.(*document.TextDocument)
	require.True(t, ok)
	assert.Equal(t, "Pick one", string(*header))
	value, ok := got.Options[0].Value.Value.(*document.TextDocument)
	require.True(t, ok)
	assert.Equal(t, "yes", string(*value))
}

func TestExternalAndSensitiveContainerPreserveNestedMediaTypes(t *testing.T) {
	RegisterContents()
	for _, test := range []struct {
		name string
		doc  document.Document
		mime document.MediaType
		want string
	}{
		{
			name: "external",
			doc:  &External{Content: document.TextDocument("outside")},
			mime: MediaTypeExternal(),
			want: `{"type":"text/plain","content":"outside"}`,
		},
		{
			name: "sensitive",
			doc:  &SensitiveContainer{Value: &document.JsonDocument{"secret": "value"}},
			mime: MediaTypeSensitiveContainer(),
			want: `{"type":"application/json","value":{"secret":"value"}}`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.doc)
			require.NoError(t, err)
			assert.JSONEq(t, test.want, string(encoded))

			decoded, err := document.UnmarshalDocument(rawJSON(encoded), test.mime)
			require.NoError(t, err)
			assert.IsType(t, test.doc, decoded)
		})
	}
}

func TestMediaLinkAndLocationWireShapes(t *testing.T) {
	linkJSON, err := json.Marshal(&MediaLink{
		Link:        Link{URI: "https://example.com/image.png", Title: "Photo"},
		Type:        pointer(document.MediaType{Type: "image", Subtype: "png"}),
		Size:        pointer(int64(128)),
		AspectRatio: "4:3",
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"uri":"https://example.com/image.png","title":"Photo","type":"image/png","size":128,"aspectRatio":"4:3"}`, string(linkJSON))

	locationJSON, err := json.Marshal(&Location{Latitude: pointer(1.5), Longitude: pointer(-2.25), Course: pointer(90)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"latitude":1.5,"longitude":-2.25,"course":90}`, string(locationJSON))
}

func TestChatStateReplyAndReactionWireShapes(t *testing.T) {
	RegisterContents()
	chatStateJSON, err := json.Marshal(&ChatState{State: ChatStateComposing, Interval: pointer(1000)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"state":"composing","interval":1000}`, string(chatStateJSON))

	replyJSON, err := json.Marshal(&Reply{
		Replied: document.NewDocumentContainer(document.TextDocument("answer")),
		InReplyTo: &InReplyTo{
			ID:        "message-1",
			Value:     document.TextDocument("question"),
			Direction: pointer(MessageDirectionReceived),
		},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"replied":{"type":"text/plain","value":"answer"},"inReplyTo":{"id":"message-1","type":"text/plain","value":"question","direction":"received"}}`, string(replyJSON))

	reactionJSON, err := json.Marshal(&Reaction{
		Emoji: &UnicodeSequence{Values: []uint32{0x1f44d}},
		InReactionTo: &InReactionTo{
			ID:        "message-1",
			Value:     document.TextDocument("question"),
			Direction: pointer(MessageDirectionSent),
		},
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{"emoji":{"values":[128077]},"inReactionTo":{"id":"message-1","type":"text/plain","value":"question","direction":"sent"}}`, string(reactionJSON))
}

func pointer[T any](value T) *T { return &value }

func rawJSON(value []byte) *json.RawMessage {
	raw := json.RawMessage(value)
	return &raw
}
