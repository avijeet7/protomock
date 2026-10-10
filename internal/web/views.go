package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/avijeet7/protomock/internal/loader"
	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// mocksView is the response of GET /api/mocks: everything the editor shows.
type mocksView struct {
	MocksDir string           `json:"mocks_dir"`
	Writable bool             `json:"writable"`
	Mocks    []mockView       `json:"mocks"`
	Problems []loader.Problem `json:"problems"`
}

type mockView struct {
	Kind       loader.Kind     `json:"kind"`
	Name       string          `json:"name"`
	ProtoFiles []protoFileView `json:"proto_files"`
	ProtoError string          `json:"proto_error,omitempty"`
	Messages   []string        `json:"messages"` // protobuf message types usable in stubs
	Methods    []methodView    `json:"methods"`  // gRPC methods defined in the protos
	Stubs      []stubView      `json:"stubs"`
}

type protoFileView struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type methodView struct {
	Path            string `json:"path"` // "/package.Service/Method", the stub's request.url
	Input           string `json:"input"`
	Output          string `json:"output"`
	ClientStreaming bool   `json:"client_streaming,omitempty"`
	ServerStreaming bool   `json:"server_streaming,omitempty"`
}

type stubView struct {
	File    string `json:"file"`
	Content string `json:"content"` // file content as-is
	Error   string `json:"error,omitempty"`
	// Parsed summary for the list (best effort, empty if the JSON is invalid).
	Method   string `json:"method,omitempty"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Status   int    `json:"status,omitempty"`
}

func newMocksView(set *loader.Set, root string, writable bool) mocksView {
	view := mocksView{MocksDir: root, Writable: writable, Mocks: []mockView{}, Problems: []loader.Problem{}}
	for _, m := range set.Mocks {
		view.Mocks = append(view.Mocks, newMockView(m))
	}
	for _, p := range set.Problems {
		view.Problems = append(view.Problems, loader.Problem{Path: relPath(root, p.Path), Error: p.Error})
	}
	return view
}

func newMockView(m *loader.Mock) mockView {
	view := mockView{
		Kind:       m.Kind,
		Name:       m.Name,
		ProtoFiles: []protoFileView{},
		ProtoError: m.ProtoError,
		Messages:   []string{},
		Methods:    []methodView{},
		Stubs:      []stubView{},
	}
	for _, name := range m.ProtoFiles {
		content, _ := os.ReadFile(filepath.Join(m.Dir, name))
		view.ProtoFiles = append(view.ProtoFiles, protoFileView{Name: name, Content: string(content)})
	}
	if m.Protos != nil {
		for _, f := range m.Protos.Files {
			collectMessages(f.Messages(), &view.Messages)
			services := f.Services()
			for i := 0; i < services.Len(); i++ {
				methods := services.Get(i).Methods()
				for j := 0; j < methods.Len(); j++ {
					md := methods.Get(j)
					view.Methods = append(view.Methods, methodView{
						Path:            "/" + string(services.Get(i).FullName()) + "/" + string(md.Name()),
						Input:           string(md.Input().FullName()),
						Output:          string(md.Output().FullName()),
						ClientStreaming: md.IsStreamingClient(),
						ServerStreaming: md.IsStreamingServer(),
					})
				}
			}
		}
		sort.Strings(view.Messages)
	}
	for _, s := range m.Stubs {
		view.Stubs = append(view.Stubs, newStubView(s))
	}
	return view
}

func collectMessages(msgs protoreflect.MessageDescriptors, out *[]string) {
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		if md.IsMapEntry() {
			continue
		}
		*out = append(*out, string(md.FullName()))
		collectMessages(md.Messages(), out)
	}
}

func newStubView(s *loader.StubFile) stubView {
	view := stubView{File: s.Name, Content: string(s.Raw), Error: s.Error}
	var stub models.Stub
	if json.Unmarshal(s.Raw, &stub) == nil {
		view.Method = stub.Request.Method
		view.URL = stub.Request.URL
		view.Priority = stub.Priority
		view.Status = stub.Response.Status
	}
	return view
}

// relPath shows paths relative to the mocks folder (stable and shorter in the UI).
func relPath(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}
