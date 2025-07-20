package models

import "encoding/json"

// Stub represents the full structure of the stub JSON file (WireMock style)
type Stub struct {
	Request  RequestStub  `json:"request"`
	Response ResponseStub `json:"response"`
}

// RequestStub defines the matching rules
type RequestStub struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    json.RawMessage   `json:"body,omitempty"`
}

// ResponseStub defines the stubbed response
type ResponseStub struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Body    json.RawMessage `json:"body"`
	Proto   bool            `json:"proto"` // true = return as Protobuf, false = JSON
}
