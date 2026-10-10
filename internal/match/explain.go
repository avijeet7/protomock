package match

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// maxReasons caps how many differences are reported for one body.
const maxReasons = 5

// ExplainJSON returns human-readable reasons why actual doesn't partially match
// expected (see PartialJSON), e.g. `body.user: expected "bob", got "alice"`.
// It returns nil when the bodies match.
func ExplainJSON(expected, actual []byte) []string {
	var exp, act any
	if err := json.Unmarshal(expected, &exp); err != nil {
		return []string{"body: the stub's request.body is not valid JSON"}
	}
	if len(actual) == 0 {
		return []string{"body: request has no body (expected " + short(exp) + ")"}
	}
	if err := json.Unmarshal(actual, &act); err != nil {
		return []string{"body: request body is not valid JSON"}
	}
	var reasons []string
	explain("body", exp, act, &reasons)
	return reasons
}

func explain(path string, exp, act any, reasons *[]string) {
	if len(*reasons) >= maxReasons {
		return
	}
	expObj, ok := exp.(map[string]any)
	if !ok {
		if !reflect.DeepEqual(exp, act) {
			*reasons = append(*reasons, fmt.Sprintf("%s: expected %s, got %s", path, short(exp), short(act)))
		}
		return
	}
	actObj, ok := act.(map[string]any)
	if !ok {
		*reasons = append(*reasons, fmt.Sprintf("%s: expected an object, got %s", path, short(act)))
		return
	}

	keys := make([]string, 0, len(expObj))
	for k := range expObj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		av, exists := actObj[k]
		if !exists {
			if len(*reasons) < maxReasons {
				*reasons = append(*reasons, fmt.Sprintf("%s.%s: missing (expected %s)", path, k, short(expObj[k])))
			}
			continue
		}
		explain(path+"."+k, expObj[k], av, reasons)
	}
}

// short renders a JSON value compactly, truncated for long values.
func short(v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 60 {
		return string(b[:57]) + "..."
	}
	return string(b)
}
