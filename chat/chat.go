package chat

import "github.com/takenet/lime-go"

func RegisterChatDocuments() {
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Account{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Contact{}
	})
}
