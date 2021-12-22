package lime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// Command allows the manipulation of node resources, like server session parameters or
// information related to the network nodes.
type Command struct {
	EnvelopeBase
	Method   CommandMethod // Method defines the action to be taken to the resource.
	Uri      *URI          // Uri is the universal identifier of the resource.
	Type     *MediaType    // Type defines MIME declaration of the resource type of the command.
	Resource Document      // Resource defines the document that is subject of the command.
	Status   CommandStatus // Status indicates the status of the action taken To the resource, in case of a response command.
	Reason   *Reason       // Reason indicates the cause for a failure response command.
}

func (c *Command) SetResource(d Document) {
	c.Resource = d
	t := d.MediaType()
	c.Type = &t
}

func (c *Command) SetStatusFailure(r Reason) {
	c.Status = CommandStatusFailure
	c.Reason = &r
}

func (c *Command) MarshalJSON() ([]byte, error) {
	raw, err := c.toRawEnvelope()
	if err != nil {
		return nil, err
	}
	return json.Marshal(raw)
}

func (c *Command) UnmarshalJSON(b []byte) error {
	raw := rawEnvelope{}
	err := json.Unmarshal(b, &raw)
	if err != nil {
		return err
	}

	command := Command{}
	err = command.populate(&raw)
	if err != nil {
		return err
	}

	*c = command
	return nil
}

func (c *Command) toRawEnvelope() (*rawEnvelope, error) {
	raw, err := c.EnvelopeBase.toRawEnvelope()
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

	if c.Status != "" {
		raw.Status = &c.Status
	}
	raw.Uri = c.Uri
	raw.Reason = c.Reason

	return raw, nil
}

func (c *Command) populate(raw *rawEnvelope) error {
	err := c.EnvelopeBase.populate(raw)
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

	if raw.Status != nil {
		c.Status = *raw.Status
	}

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
	// If the resource value is absent, it represents that the resource in the specified URI was deleted in the originator.
	// This method can be one way and the destination may not send a response for it.
	// Because of that, a command envelope with this method may not have an ID.
	CommandMethodObserve = CommandMethod("observe")
	// CommandMethodMerge Merge a resource document with an existing one. If the resource doesn't exist, it is created.
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

const URISchemeLime = "lime"

type URI struct {
	url *url.URL
}

func (u URI) ToURL() url.URL {
	return *u.url
}

func ParseLimeURI(s string) (URI, error) {
	u, err := url.Parse(s)
	if err != nil {
		return URI{}, err
	}

	if u.IsAbs() && u.Scheme != URISchemeLime {
		return URI{}, fmt.Errorf("invalid scheme '%v'", u.Scheme)
	}

	return URI{u}, nil
}

func (u URI) MarshalText() ([]byte, error) {
	if u.url == nil {
		return nil, nil
	}

	return []byte(u.url.String()), nil
}

func (u *URI) UnmarshalText(text []byte) error {
	uri, err := ParseLimeURI(string(text))
	if err != nil {
		return err
	}
	*u = uri
	return nil
}
