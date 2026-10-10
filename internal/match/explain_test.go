package match

import (
	"strings"
	"testing"
)

func TestExplainJSON(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		want     []string // nil = match
	}{
		{"match", `{"user":"bob"}`, `{"user":"bob","x":1}`, nil},
		{"different value", `{"user":"bob"}`, `{"user":"alice"}`, []string{`body.user: expected "bob", got "alice"`}},
		{"missing field", `{"user":"bob"}`, `{}`, []string{`body.user: missing (expected "bob")`}},
		{"nested", `{"a":{"b":1}}`, `{"a":{"b":2}}`, []string{`body.a.b: expected 1, got 2`}},
		{"object vs scalar", `{"a":{"b":1}}`, `{"a":"x"}`, []string{`body.a: expected an object, got "x"`}},
		{"array differs", `{"tags":["a"]}`, `{"tags":["b"]}`, []string{`body.tags: expected ["a"], got ["b"]`}},
		{"empty request", `{"a":1}`, ``, []string{`body: request has no body (expected {"a":1})`}},
		{"invalid request", `{"a":1}`, `nope`, []string{`body: request body is not valid JSON`}},
		{"several, sorted", `{"b":1,"a":1}`, `{}`, []string{`body.a: missing (expected 1)`, `body.b: missing (expected 1)`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExplainJSON([]byte(tt.expected), []byte(tt.actual))
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("ExplainJSON = %q, want %q", got, tt.want)
			}
			// ExplainJSON must agree with PartialJSON on whether the bodies match.
			if matched := PartialJSON([]byte(tt.expected), []byte(tt.actual)); matched != (got == nil) {
				t.Errorf("PartialJSON = %v but ExplainJSON = %q", matched, got)
			}
		})
	}
}

func TestExplainJSON_CapsReasons(t *testing.T) {
	got := ExplainJSON([]byte(`{"a":1,"b":1,"c":1,"d":1,"e":1,"f":1,"g":1}`), []byte(`{}`))
	if len(got) != maxReasons {
		t.Errorf("got %d reasons, want %d", len(got), maxReasons)
	}
}
