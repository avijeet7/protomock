package grpcserver

import (
	"encoding/json"
)

// partialJSONMatch checks whether all fields in `expected` are present in `actual`
func partialJSONMatch(expectedRaw, actualRaw []byte) bool {
	var expected, actual map[string]interface{}
	_ = json.Unmarshal(expectedRaw, &expected)
	_ = json.Unmarshal(actualRaw, &actual)

	for k, v := range expected {
		if actual[k] != v {
			return false
		}
	}
	return true
}
