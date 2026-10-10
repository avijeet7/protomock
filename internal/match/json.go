// Package match implements request matching rules shared by the HTTP and gRPC servers.
package match

import (
	"encoding/json"
	"reflect"
)

// PartialJSON reports whether actual contains everything in expected
// (WireMock-style partial matching):
//   - objects: every key in expected must exist in actual and match recursively;
//     extra keys in actual are ignored
//   - arrays and scalars: must be equal
//
// Invalid JSON on either side never matches.
func PartialJSON(expected, actual []byte) bool {
	var exp, act interface{}
	if err := json.Unmarshal(expected, &exp); err != nil {
		return false
	}
	if err := json.Unmarshal(actual, &act); err != nil {
		return false
	}
	return partial(exp, act)
}

func partial(exp, act interface{}) bool {
	expObj, ok := exp.(map[string]interface{})
	if !ok {
		return reflect.DeepEqual(exp, act)
	}

	actObj, ok := act.(map[string]interface{})
	if !ok {
		return false
	}
	for k, v := range expObj {
		av, exists := actObj[k]
		if !exists || !partial(v, av) {
			return false
		}
	}
	return true
}
