package lime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// Command Allows the manipulation of node resources, like server session parameters or
// information related to the network nodes.
type Command struct {
	EnvelopeBase

	// Action To be taken To the resource.
	Method CommandMethod

	// The universal identifier of the resource.
	Uri *LimeUri

	// MIME declaration of the resource type of the command.
	Type *MediaType

	// Node resource that is subject of the command.
	Resource Document

	// Indicates the status of the action taken To the resource, in case of
	// a response command.
	Status CommandStatus

	// Indicates the reason for a failure response command.
	Reason *Reason
}

func (c *Command) SetResource(d Document) {
	c.Resource = d
	t := d.GetMediaType()
	c.Type = &t
}

func (c *Command) SetStatusFailure(r Reason) {
	c.Status = CommandStatusFailure
	c.Reason = &r
}

func (c *Command) MarshalJSON() ([]byte, error) {
	raw, err := c.ToRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (c *Command) UnmarshalJSON(b []byte) error {
	raw := RawEnvelope{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	command := Command{}
	err = command.Populate(&raw)
	if err != nil {
		return err
	}

	*c = command
	return nil
}

func (c *Command) ToRawEnvelope() (*RawEnvelope, error) {
	raw, err := c.EnvelopeBase.ToRawEnvelope()
	if err != nil {
		return nil, err
	}

	if c.Resource != nil {
		b, err := json.Marshal(c.Resource)
		if err != nil {
			return nil, err
		}
		r := json.RawMessage(b)
		raw.Resource = &r
		raw.Type = c.Type
	}
	if c.Method != "" {
		raw.Method = &c.Method
	}

	raw.Status = &c.Status
	raw.Uri = c.Uri
	raw.Reason = c.Reason

	return raw, nil
}

func (c *Command) Populate(raw *RawEnvelope) error {
	err := c.EnvelopeBase.Populate(raw)
	if err != nil {
		return err
	}

	// Create the document type instance and unmarshal the json to it
	if raw.Resource != nil {
		if raw.Type == nil {
			return errors.New("command resource type is required when resource is present")
		}

		document, err := UnmarshalDocument(raw.Resource, *raw.Type)
		if err != nil {
			return err
		}

		c.Resource = document
		c.Type = raw.Type
	}

	if raw.Method == nil {
		return errors.New("command method is required")
	}
	c.Method = *raw.Method
	c.Status = *raw.Status
	c.Uri = raw.Uri
	c.Reason = raw.Reason

	return nil
}

// CommandMethod Defines methods for the manipulation of resources.
type CommandMethod string

const (
	// CommandMethodGet Get an existing value of a resource.
	CommandMethodGet = CommandMethod("get")
	// CommandMethodSet Create or updates the value of a resource.
	CommandMethodSet = CommandMethod("set")
	// CommandMethodDelete Delete a value of the resource or the resource itself.
	CommandMethodDelete = CommandMethod("delete")
	// CommandMethodSubscribe Subscribe To a resource, allowing the originator To be notified when the
	// value of the resource changes in the destination.
	CommandMethodSubscribe = CommandMethod("subscribe")
	// CommandMethodUnsubscribe Unsubscribe To the resource, signaling To the destination that the
	// originator do not want To receive further notifications about the resource.
	CommandMethodUnsubscribe = CommandMethod("unsubscribe")
	// CommandMethodObserve Notify the destination about a change in a resource value of the sender.
	// If the resource value is absent, it represent that the resource in the specified URI was deleted in the originator.
	// This method can be one way and the destination may not send a response for it.
	// Because of that, a command envelope with this method may not have an ID.
	CommandMethodObserve = CommandMethod("observe")
	// CommandMethodMerge Merge a resource document with an existing one. If the resource doesn't exists, it is created.
	CommandMethodMerge = CommandMethod("merge")
)

func (m CommandMethod) IsValid() error {
	switch m {
	case CommandMethodGet, CommandMethodSet, CommandMethodDelete, CommandMethodSubscribe, CommandMethodUnsubscribe, CommandMethodObserve, CommandMethodMerge:
		return nil
	}

	return fmt.Errorf("invalid command method '%v'", m)
}

func (m CommandMethod) MarshalText() ([]byte, error) {
	err := m.IsValid()
	if err != nil {
		return []byte{}, err
	}
	return []byte(m), nil
}

func (m *CommandMethod) UnmarshalText(text []byte) error {
	method := CommandMethod(text)
	err := method.IsValid()
	if err != nil {
		return err
	}
	*m = method
	return nil
}

type CommandStatus string

const (
	CommandStatusSuccess = CommandStatus("success")
	CommandStatusFailure = CommandStatus("failure")
)

const UriSchemeLime = "lime"

type LimeUri struct {
	url *url.URL
}

func (u LimeUri) ToURL() url.URL {
	return *u.url
}

func ParseLimeUri(s string) (LimeUri, error) {
	u, err := url.Parse(s)
	if err != nil {
		return LimeUri{}, err
	}

	if u.IsAbs() && u.Scheme != UriSchemeLime {
		return LimeUri{}, fmt.Errorf("invalid scheme '%v'", u.Scheme)
	}

	return LimeUri{u}, nil
}

func (u LimeUri) MarshalText() ([]byte, error) {
	if u.url == nil {
		return nil, nil
	}

	return []byte(u.url.String()), nil
}

func (u *LimeUri) UnmarshalText(text []byte) error {
	uri, err := ParseLimeUri(string(text))
	if err != nil {
		return err
	}
	*u = uri
	return nil
}
