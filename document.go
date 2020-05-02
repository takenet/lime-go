package lime

import (
	"errors"
	"fmt"
	"strings"
)

type Document interface {
	GetMediaType() MediaType
}

type MediaType struct {
	Type    string
	Subtype string
	Suffix  string
}

func (m MediaType) IsJson() bool {
	return m.Suffix == "json"
}

func (m MediaType) String() string {
	if m == (MediaType{}) {
		return ""
	}

	v := fmt.Sprintf("%v/%v", m.Type, m.Subtype)
	if m.Suffix != "" {
		return fmt.Sprintf("%v+%v", v, m.Suffix)
	}

	return v
}

func ParseMediaType(s string) (MediaType, error) {
	var suffix string
	values := strings.Split(s, "+")
	if len(values) > 1 {
		suffix = values[1]
	}
	values = strings.Split(values[0], "/")

	if len(values) == 1 {
		return MediaType{}, errors.New("invalid media type")
	}

	return MediaType{values[0], values[1], suffix}, nil
}

func (m MediaType) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

func (m *MediaType) UnmarshalText(text []byte) error {
	mediaType, err := ParseMediaType(string(text))
	if err != nil {
		return err
	}
	*m = mediaType
	return nil
}

var mediaTypeApplicationJson MediaType = MediaType{"application", "json", ""}
var mediaTypeTextPlain MediaType = MediaType{"text", "plain", ""}

type JsonDocument map[string]interface{}

func (d *JsonDocument) GetMediaType() MediaType {
	return mediaTypeApplicationJson
}

type PlainDocument string

func (d *PlainDocument) GetMediaType() MediaType {
	return mediaTypeTextPlain
}

var DocumentFactories = map[MediaType]func() Document{
	mediaTypeTextPlain: func() Document {
		d := PlainDocument("")
		return &d
	},
	mediaTypeApplicationJson: func() Document {
		return &JsonDocument{}
	},
}
