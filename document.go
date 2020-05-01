package main

type MediaType struct {
}

type Document interface {
	getMediaType() MediaType
}
