package lime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
)

// Allows the manipulation of node resources, like server session parameters or
// information related To the network nodes.
type Command struct {
	EnvelopeBase

	// Action To be taken To the resource.
	Method CommandMethod

	// The universal identifier of the resource.
	Uri LimeUri

	// MIME declaration of the resource type of the command.
	Type MediaType

	// Node resource that is subject of the command.
	Resource Document

	// Indicates the status of the action taken To the resource, in case of
	// a response command.
	Status CommandStatus

	// Indicates the reason for a failure response command.
	Reason Reason
}

func (c *Command) SetResource(d Document) {
	c.Resource = d
	c.Type = d.GetMediaType()
}

func (c *Command) SetStatusFailure(r Reason) {
	c.Status = CommandStatusFailure
	c.Reason = r
}

// Wrapper for custom marshalling
type CommandWrapper struct {
	EnvelopeBaseWrapper
	Method   CommandMethod    `json:"method"`
	Uri      *LimeUri         `json:"uri,omitempty"`
	Type     *MediaType       `json:"type,omitempty"`
	Resource *json.RawMessage `json:"resource,omitempty"`
	Status   CommandStatus    `json:"status,omitempty"`
	Reason   *Reason          `json:"reason,omitempty"`
}

func (c Command) MarshalJSON() ([]byte, error) {
	cw, err := c.toWrapper()
	if err != nil {
		return nil, err
	}
	return json.Marshal(cw)
}

func (c *Command) UnmarshalJSON(b []byte) error {
	cw := CommandWrapper{}
	err := json.Unmarshal(b, &cw)
	if err != nil {
		return err
	}

	command := Command{}
	err = command.populate(&cw)
	if err != nil {
		return err
	}

	*c = command
	return nil
}

func (c *Command) toWrapper() (CommandWrapper, error) {
	ew, err := c.EnvelopeBase.toWrapper()
	if err != nil {
		return CommandWrapper{}, err
	}

	cw := CommandWrapper{
		EnvelopeBaseWrapper: ew,
	}

	if c.Resource != nil {
		b, err := json.Marshal(c.Resource)
		if err != nil {
			return CommandWrapper{}, err
		}
		r := json.RawMessage(b)
		cw.Resource = &r
		cw.Type = &c.Type
	}

	cw.Method = c.Method
	cw.Status = c.Status
	if c.Uri != (LimeUri{}) {
		cw.Uri = &c.Uri
	}
	if c.Reason != (Reason{}) {
		cw.Reason = &c.Reason
	}

	return cw, nil
}

func (c *Command) populate(cw *CommandWrapper) error {
	err := c.EnvelopeBase.populate(&cw.EnvelopeBaseWrapper)
	if err != nil {
		return err
	}

	// Create the document type instance and unmarshal the json to it
	if cw.Resource != nil {
		if cw.Type == nil {
			return errors.New("command resource type is required when resource is present")
		}

		document, err := UnmarshalDocument(cw.Resource, *cw.Type)
		if err != nil {
			return err
		}

		c.Resource = document
		c.Type = *cw.Type
	}

	c.Method = cw.Method
	c.Status = cw.Status
	if cw.Uri != nil {
		c.Uri = *cw.Uri
	}
	if cw.Reason != nil {
		c.Reason = *cw.Reason
	}

	return nil
}

// Defines methods for the manipulation of resources.
type CommandMethod string

const (
	// Get an existing value of a resource.
	CommandMethodGet = CommandMethod("get")
	// Create or updates the value of a resource.
	CommandMethodSet = CommandMethod("set")
	// Delete a value of the resource or the resource itself.
	CommandMethodDelete = CommandMethod("delete")
	// Subscribe To a resource, allowing the originator To be notified when the
	// value of the resource changes in the destination.
	CommandMethodSubscribe = CommandMethod("subscribe")
	// Unsubscribe To the resource, signaling To the destination that the
	// originator do not want To receive further notifications about the resource.
	CommandMethodUnsubscribe = CommandMethod("unsubscribe")
	// Notify the destination about a change in a resource value of the sender.
	// If the resource value is absent, it represent that the resource in the specified URI was deleted in the originator.
	// This method can be one way and the destination may not send a response for it.
	// Because of that, a command envelope with this method may not have an ID.
	CommandMethodObserve = CommandMethod("observe")
	// Merge a resource document with an existing one. If the resource doesn't exists, it is created.
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
