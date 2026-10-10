package match

import "testing"

func TestPartialJSON(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		want     bool
	}{
		{"flat partial match", `{"name":"bob"}`, `{"name":"bob","age":3}`, true},
		{"flat mismatch", `{"name":"bob"}`, `{"name":"alice"}`, false},
		{"nested object", `{"user":{"id":"1"}}`, `{"user":{"id":"1","name":"bob"}}`, true},
		{"repeated field", `{"tags":["a","b"]}`, `{"tags":["a","b"]}`, true},
		{"repeated field order matters", `{"tags":["a","b"]}`, `{"tags":["b","a"]}`, false},
		{"nested object inside array", `{"items":[{"id":1}]}`, `{"items":[{"id":1}]}`, true},
		{"expected object, got scalar", `{"user":{"id":"1"}}`, `{"user":"1"}`, false},
		{"invalid expected JSON", `{nope`, `{"a":1}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PartialJSON([]byte(tt.expected), []byte(tt.actual)); got != tt.want {
				t.Errorf("PartialJSON(%s, %s) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}
