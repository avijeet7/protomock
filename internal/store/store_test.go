package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/avijeet7/protomock/internal/loader"
)

func TestStubPath_RejectsUnsafeNames(t *testing.T) {
	s := &Store{Root: "/mocks"}
	tests := []struct {
		name       string
		kind       loader.Kind
		mock, file string
		want       string // empty = must be rejected
	}{
		{"valid", loader.KindHTTP, "users", "list.json", "/mocks/http/users/stubs/list.json"},
		{"nested mock", loader.KindGRPC, "team/users", "get_1.json", "/mocks/grpc/team/users/stubs/get_1.json"},
		{"unknown kind", "ftp", "users", "list.json", ""},
		{"empty mock", loader.KindHTTP, "", "list.json", ""},
		{"parent dir in mock", loader.KindHTTP, "../etc", "list.json", ""},
		{"dot segment", loader.KindHTTP, "users/./x", "list.json", ""},
		{"absolute mock", loader.KindHTTP, "/etc", "list.json", ""},
		{"backslash", loader.KindHTTP, `users\..\x`, "list.json", ""},
		{"file with slash", loader.KindHTTP, "users", "../list.json", ""},
		{"not json", loader.KindHTTP, "users", "list.txt", ""},
		{"hidden file", loader.KindHTTP, "users", ".list.json", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.StubPath(tt.kind, tt.mock, tt.file)
			if tt.want == "" {
				if !errors.Is(err, ErrInvalidName) {
					t.Errorf("StubPath(%q, %q, %q) = %q, %v; want ErrInvalidName", tt.kind, tt.mock, tt.file, got, err)
				}
				return
			}
			if err != nil || got != filepath.FromSlash(tt.want) {
				t.Errorf("StubPath = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestWriteFileAndDelete(t *testing.T) {
	s := &Store{Root: t.TempDir()}
	if err := s.CreateMock(loader.KindHTTP, "users"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateMock(loader.KindHTTP, "users"); err == nil {
		t.Error("creating an existing mock should fail")
	}

	path, _ := s.StubPath(loader.KindHTTP, "users", "list.json")
	if err := WriteFile(path, []byte(`{"a":1}`)); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != `{"a":1}` {
		t.Errorf("content = %s", got)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".protomock-*"))
	if len(leftovers) > 0 {
		t.Errorf("temp files left behind: %v", leftovers)
	}

	if err := Delete(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file still exists after Delete")
	}
	if err := Delete(filepath.Dir(path)); err == nil {
		t.Error("Delete must refuse folders")
	}
}

func TestWritable(t *testing.T) {
	dir := t.TempDir()
	if !(&Store{Root: dir}).Writable() {
		t.Error("temp dir should be writable")
	}
	if !(&Store{Root: filepath.Join(dir, "not-yet-created")}).Writable() {
		t.Error("a missing mocks folder in a writable parent counts as writable")
	}
	if os.Getuid() != 0 { // root ignores permissions
		ro := filepath.Join(dir, "ro")
		if err := os.Mkdir(ro, 0o555); err != nil {
			t.Fatal(err)
		}
		if (&Store{Root: ro}).Writable() {
			t.Error("read-only folder reported as writable")
		}
	}
}
