package models

import (
	"encoding/json"
	"regexp"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Route represents a fully loaded mock route after parsing proto + stub
type Route struct {
	URL          string
	Method       string
	HeaderMatch  map[string]string
	BodyMatch    json.RawMessage
	Status       int
	MessageDesc  protoreflect.MessageDescriptor // response message type
	RequestDesc  protoreflect.MessageDescriptor // gRPC request message type, from the service definition (nil for HTTP)
	Message      *dynamicpb.Message             // response message
	ProtoEncoded bool
	RawJSONBody  []byte
	Mock         string         // Mock folder name (relative to <mocks>/<kind>)
	FilePath     string         // Path to the stub file
	PathRegex    *regexp.Regexp // Compiled URL pattern for regex routes (nil for exact routes)
}
