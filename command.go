package lime

import (
	"fmt"
	"net/url"
)

// Allows the manipulation of node resources, like server session parameters or
// information related to the network nodes.
type Command struct {
	Envelope
	// Action to be taken to the resource.
	Method CommandMethod `json:"method"`
	// The universal identifier of the resource.
	Uri LimeUri `json:"uri,omitempty"`
	// MIME declaration of the resource type of the command.
	Type MediaType `json:"type,omitempty"`
	// Node resource that is subject of the command.
	Resource Document `json:"resource,omitempty"`
	// Indicates the status of the action taken to the resource, in case of
	// a response command.
	Status CommandStatus `json:"resource,omitempty"`
	// Indicates the reason for a failure response command.
	Reason *Reason `json:"reason,omitempty"`
}

func (c *Command) SetResource(d Document) {
	c.Resource = d
	c.Type = d.GetMediaType()
}

func (c *Command) SetStatusFailure(r *Reason) {
	c.Status = CommandStatusFailure
	c.Reason = r
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
	// Subscribe to a resource, allowing the originator to be notified when the
	// value of the resource changes in the destination.
	CommandMethodSubscribe = CommandMethod("subscribe")
	// Unsubscribe to the resource, signaling to the destination that the
	// originator do not want to receive further notifications about the resource.
	CommandMethodUnsubscribe = CommandMethod("unsubscribe")
	// Notify the destination about a change in a resource value of the sender.
	// If the resource value is absent, it represent that the resource in the specified URI was deleted in the originator.
	// This method can be one way and the destination may not send a response for it.
	// Because of that, a command envelope with this method may not have an id.
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
