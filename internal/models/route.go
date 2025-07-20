package models

import (
	"encoding/json"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Route represents a fully loaded mock route after parsing proto + stub
type Route struct {
	URL          string
	Method       string
	HeaderMatch  map[string]string
	BodyMatch    json.RawMessage
	Status       int
	MessageDesc  *desc.MessageDescriptor
	Message      *dynamic.Message // response message
	ProtoEncoded bool
}
