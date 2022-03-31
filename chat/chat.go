package chat

import "github.com/takenet/lime-go"

// RegisterChatDocuments register all documents from the chat package in the Lime's document registry.
func RegisterChatDocuments() {
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Account{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Contact{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Delegation{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Presence{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Receipt{}
	})
}
