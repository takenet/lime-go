package resources

import (
	"time"

	"github.com/takenet/lime-go/document"
	lime "github.com/takenet/lime-go/protocol"
)

type Group struct {
	Identity    lime.Identity `json:"identity,omitempty"`
	Name        string        `json:"name,omitempty"`
	Description string        `json:"description,omitempty"`
	Type        *GroupType    `json:"type,omitempty"`
	PhotoURI    string        `json:"photoUri,omitempty"`
	Creator     lime.Identity `json:"creator,omitempty"`
	Created     time.Time     `json:"created,omitempty"`
	MembersURI  *lime.URI     `json:"members,omitempty"`
}

func MediaTypeGroup() document.MediaType {
	return document.MediaType{Type: document.MediaTypeApplication, Subtype: "vnd.lime.group", Suffix: "json"}
}

func (*Group) MediaType() document.MediaType { return MediaTypeGroup() }

type GroupType string

const (
	GroupTypeTemporary GroupType = "temporary"
	GroupTypePrivate   GroupType = "private"
	GroupTypePublic    GroupType = "public"
)

type GroupMember struct {
	Address lime.Node       `json:"address"`
	Role    GroupMemberRole `json:"role,omitempty"`
}

func MediaTypeGroupMember() document.MediaType {
	return document.MediaType{Type: document.MediaTypeApplication, Subtype: "vnd.lime.groupmember", Suffix: "json"}
}

func (*GroupMember) MediaType() document.MediaType { return MediaTypeGroupMember() }

type GroupMemberRole string

const (
	GroupMemberRoleMember    GroupMemberRole = "member"
	GroupMemberRoleListener  GroupMemberRole = "listener"
	GroupMemberRoleModerator GroupMemberRole = "moderator"
	GroupMemberRoleOwner     GroupMemberRole = "owner"
)

type Capability struct {
	ContentTypes  []document.MediaType `json:"contentTypes,omitempty"`
	ResourceTypes []document.MediaType `json:"resourceTypes,omitempty"`
}

func MediaTypeCapability() document.MediaType {
	return document.MediaType{Type: document.MediaTypeApplication, Subtype: "vnd.lime.capability", Suffix: "json"}
}

func (*Capability) MediaType() document.MediaType { return MediaTypeCapability() }

type Quota struct {
	Threshold       *int `json:"threshold,omitempty"`
	Throughput      *int `json:"throughput,omitempty"`
	MaxEnvelopeSize *int `json:"maxEnvelopeSize,omitempty"`
}

func MediaTypeQuota() document.MediaType {
	return document.MediaType{Type: document.MediaTypeApplication, Subtype: "vnd.lime.quota", Suffix: "json"}
}

func (*Quota) MediaType() document.MediaType { return MediaTypeQuota() }
