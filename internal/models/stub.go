package models

import "encoding/json"

// Stub is the structure of a stub JSON file (WireMock style).
type Stub struct {
	// Priority orders stubs that could match the same request: higher is tried first.
	// Stubs with equal priority are tried in file-name order. Default 0.
	Priority int          `json:"priority,omitempty"`
	Request  RequestStub  `json:"request"`
	Response ResponseStub `json:"response"`
}

// RequestStub defines the matching rules.
type RequestStub struct {
	Method  string            `json:"method,omitempty"` // HTTP only
	URL     string            `json:"url"`              // HTTP path or regex; gRPC "/package.Service/Method"
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"` // partial JSON match
}

// ResponseStub defines the stubbed response.
type ResponseStub struct {
	Status  int               `json:"status,omitempty"`  // HTTP status, default 200 (ignored for gRPC)
	Headers map[string]string `json:"headers,omitempty"` // HTTP headers / gRPC response metadata
	Message string            `json:"message,omitempty"` // protobuf message type, e.g. "pkg.Reply"
	Body    json.RawMessage   `json:"body,omitempty"`
	Proto   bool              `json:"proto,omitempty"`    // HTTP: encode body as protobuf (gRPC always is)
	DelayMs int               `json:"delay_ms,omitempty"` // wait before responding
}
