//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// writeFiles creates files (relative path -> content) under root, making parent dirs as needed.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// compileProto compiles a single in-memory .proto file so tests can build and decode messages
// independently of the server's own proto handling.
func compileProto(t *testing.T, name, source string) protoreflect.FileDescriptor {
	t.Helper()
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			Accessor: protocompile.SourceAccessorFromMap(map[string]string{name: source}),
		}),
	}
	files, err := compiler.Compile(context.Background(), name)
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return files[0]
}
