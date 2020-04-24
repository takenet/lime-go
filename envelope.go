package main

type Envelope struct {
	ID       string            `json:"id,omitempty"`
	From     *Node             `json:"from,omitempty"`
	To       *Node             `json:"to,omitempty"`
	Pp       *Node             `json:"pp,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
