package resources

import "github.com/takenet/lime-go/document"

// RegisterResources registers the standard resource document factories.
func RegisterResources() {
	document.RegisterDocumentFactory(func() document.Document {
		return &Account{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Contact{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Delegation{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Presence{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Receipt{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Group{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &GroupMember{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Capability{}
	})
	document.RegisterDocumentFactory(func() document.Document {
		return &Quota{}
	})
}
