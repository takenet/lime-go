package messaging

import "github.com/takenet/lime-go"

func RegisterMessagingDocuments() {
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Account{}
	})
	lime.RegisterDocumentFactory(func() lime.Document {
		return &Contact{}
	})
}
