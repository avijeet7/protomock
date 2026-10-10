//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
)

var codeBlockRE = regexp.MustCompile("(?s)```(json|proto)\n(.*?)```")

// TestDocs_StubExamplesAreValid checks every JSON stub example in docs/stubs.md against the
// real stub rules, using the .proto examples from the same section. It keeps the docs honest.
func TestDocs_StubExamplesAreValid(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "stubs.md"))
	if err != nil {
		t.Fatal(err)
	}
	sections := map[loader.Kind]string{
		loader.KindHTTP: section(string(doc), "## HTTP examples", "## gRPC examples"),
		loader.KindGRPC: section(string(doc), "## gRPC examples", "## Validation errors"),
	}

	for kind, text := range sections {
		dir := t.TempDir()
		var protoNames []string
		var stubs []string
		for i, m := range codeBlockRE.FindAllStringSubmatch(text, -1) {
			if m[1] == "proto" {
				name := filepath.Join(dir, "example_"+string(rune('a'+i))+".proto")
				if err := os.WriteFile(name, []byte(m[2]), 0o644); err != nil {
					t.Fatal(err)
				}
				protoNames = append(protoNames, filepath.Base(name))
			} else {
				stubs = append(stubs, m[2])
			}
		}
		var protos *loader.Protos
		if len(protoNames) > 0 {
			if protos, err = loader.CompileProtos(dir, protoNames); err != nil {
				t.Fatalf("%s: example .proto doesn't compile: %v", kind, err)
			}
		}
		if len(stubs) < 3 {
			t.Fatalf("%s: found only %d JSON examples; did the doc structure change?", kind, len(stubs))
		}
		for _, raw := range stubs {
			var stub models.Stub
			if err := json.Unmarshal([]byte(raw), &stub); err != nil {
				t.Errorf("%s example is not valid JSON: %v\n%s", kind, err, raw)
				continue
			}
			if _, err := loader.BuildRoute(kind, stub, protos); err != nil {
				t.Errorf("%s example is not a valid stub: %v\n%s", kind, err, raw)
			}
		}
	}
}

func section(doc, start, end string) string {
	i := strings.Index(doc, start)
	j := strings.Index(doc, end)
	if i < 0 || j < i {
		return ""
	}
	return doc[i:j]
}
