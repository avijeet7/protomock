package models

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

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
	Headers      map[string]string              // response headers (HTTP) / metadata (gRPC)
	Delay        time.Duration                  // wait before responding
	Priority     int                            // higher is tried first
	MessageDesc  protoreflect.MessageDescriptor // response message type
	RequestDesc  protoreflect.MessageDescriptor // gRPC request message type, from the service definition (nil for HTTP)
	Message      *dynamicpb.Message             // response message
	ProtoEncoded bool
	RawJSONBody  []byte
	Mock         string         // Mock folder name (relative to <mocks>/<kind>)
	FilePath     string         // Path to the stub file
	PathRegex    *regexp.Regexp // Compiled URL pattern for regex routes (nil for exact routes)
}

// WaitDelay sleeps for the route's response delay. It returns false if ctx is
// cancelled first (e.g. the client gave up), in which case no response should be sent.
func (r Route) WaitDelay(ctx context.Context) bool {
	if r.Delay <= 0 {
		return true
	}
	timer := time.NewTimer(r.Delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
