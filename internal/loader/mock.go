package loader

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

// Mock is one mock folder: its proto files and stubs.
type Mock struct {
	Kind       Kind
	Name       string   // path relative to <mocks>/<kind>, slash-separated
	Dir        string   // absolute or root-relative folder path
	ProtoFiles []string // .proto file names directly in Dir
	Protos     *Protos
	ProtoError string // compile error for ProtoFiles, if any
	Stubs      []*StubFile
}

// StubFile is one stub JSON file and the route built from it.
type StubFile struct {
	Name  string          // file name, e.g. "hello.json"
	Path  string          // full path
	Raw   json.RawMessage // file content as-is
	Route *models.Route   // nil if the stub is invalid
	Error string          // why the stub is invalid
}

// LoadMock loads one mock folder (<root>/<kind>/<name>).
func LoadMock(root string, kind Kind, name string) *Mock {
	mock := &Mock{Kind: kind, Name: name, Dir: filepath.Join(root, string(kind), filepath.FromSlash(name))}

	mock.ProtoFiles = listFiles(mock.Dir, ".proto")
	if len(mock.ProtoFiles) > 0 {
		files, err := CompileProtos(mock.Dir, mock.ProtoFiles)
		if err != nil {
			mock.ProtoError = err.Error()
		} else {
			mock.Protos = files
		}
	}

	stubDir := filepath.Join(mock.Dir, "stubs")
	for _, file := range listFiles(stubDir, ".json") {
		path := filepath.Join(stubDir, file)
		stub := &StubFile{Name: file, Path: path}
		mock.Stubs = append(mock.Stubs, stub)

		raw, err := os.ReadFile(path)
		if err != nil {
			stub.Error = err.Error()
			continue
		}
		stub.Raw = raw
		route, err := mock.BuildRoute(raw, path)
		if err != nil {
			stub.Error = err.Error()
			continue
		}
		stub.Route = &route
	}
	return mock
}

// BuildRoute validates a stub's JSON against this mock (its kind and protos) and builds its route.
func (m *Mock) BuildRoute(raw []byte, path string) (models.Route, error) {
	var stub models.Stub
	if err := json.Unmarshal(raw, &stub); err != nil {
		return models.Route{}, &StubError{Msg: "invalid JSON: " + err.Error()}
	}
	if m.ProtoError != "" && (m.Kind == KindGRPC || stub.Response.Proto) {
		return models.Route{}, &StubError{Msg: "the mock's .proto files don't compile (see the proto error)"}
	}
	route, err := BuildRoute(m.Kind, stub, m.Protos)
	if err != nil {
		return models.Route{}, err
	}
	route.Mock = m.Name
	route.FilePath = path
	return route, nil
}

// Problems lists the proto and stub errors in this mock.
func (m *Mock) Problems() []Problem {
	var problems []Problem
	if m.ProtoError != "" {
		problems = append(problems, Problem{Path: m.Dir, Error: m.ProtoError})
	}
	for _, s := range m.Stubs {
		if s.Error != "" {
			problems = append(problems, Problem{Path: s.Path, Error: s.Error})
		}
	}
	return problems
}

// listFiles returns the sorted names of regular files in dir with the given extension.
func listFiles(dir, ext string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
