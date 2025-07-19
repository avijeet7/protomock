package models

import (
	"encoding/json"

	"github.com/jhump/protoreflect/dynamic"
)

// Stub represents a single mock response configuration loaded from a .json file.
type Stub struct {
	URL      string          `json:"url"`      // The endpoint to serve
	Status   int             `json:"status"`   // HTTP status code or gRPC status code (if needed)
	Message  string          `json:"message"`  // Fully qualified Protobuf message name
	Response json.RawMessage `json:"response"` // JSON-formatted payload to convert into Protobuf
}

// Route holds a parsed stub with the dynamic Protobuf message, ready to be served.
type Route struct {
	URL     string           // Endpoint path or gRPC method name
	Status  int              // Response status code
	Message *dynamic.Message // Parsed Protobuf message
}
