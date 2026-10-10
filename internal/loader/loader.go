// Package loader reads a mocks folder into routes for the HTTP and gRPC servers.
//
// Layout:
//
//	<mocks>/
//	├── http/<mock>/          HTTP mocks
//	│   ├── *.proto           optional: needed only for protobuf responses
//	│   └── stubs/*.json
//	└── grpc/<mock>/          gRPC mocks
//	    ├── *.proto           service + message definitions
//	    └── stubs/*.json
//
// A mock folder is any folder (at any depth under http/ or grpc/) that contains
// a stubs/ folder or .proto files. Problems with individual protos or stubs are
// collected in Set.Problems instead of failing the whole load.
package loader

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/avijeet7/protomock/internal/models"
)

// Kind is the protocol a mock folder serves.
type Kind string

const (
	KindHTTP Kind = "http"
	KindGRPC Kind = "grpc"
)

// Kinds lists every mock kind, in display order.
var Kinds = []Kind{KindHTTP, KindGRPC}

// Set is everything loaded from a mocks folder.
type Set struct {
	Mocks    []*Mock
	HTTP     []models.Route
	GRPC     []models.Route
	Problems []Problem
}

// Problem describes a proto or stub file that could not be loaded.
type Problem struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// LoadAll loads every mock folder under root. A missing root or kind folder is not an error.
func LoadAll(root string) (*Set, error) {
	set := &Set{}
	for _, kind := range Kinds {
		names, err := findMocks(filepath.Join(root, string(kind)))
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			mock := LoadMock(root, kind, name)
			set.Mocks = append(set.Mocks, mock)
			set.Problems = append(set.Problems, mock.Problems()...)
			for _, stub := range mock.Stubs {
				if stub.Route == nil {
					continue
				}
				if kind == KindHTTP {
					set.HTTP = append(set.HTTP, *stub.Route)
				} else {
					set.GRPC = append(set.GRPC, *stub.Route)
				}
			}
		}
	}
	return set, nil
}

// findMocks returns the mock folder names (slash-separated paths relative to kindDir), sorted.
func findMocks(kindDir string) ([]string, error) {
	var names []string
	err := filepath.WalkDir(kindDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == kindDir {
				return filepath.SkipAll
			}
			return err
		}
		if !d.IsDir() || path == kindDir {
			return nil
		}
		if isMockDir(path) {
			rel, _ := filepath.Rel(kindDir, path)
			names = append(names, filepath.ToSlash(rel))
			return filepath.SkipDir // a mock's subfolders hold its stubs and imported protos
		}
		return nil
	})
	sort.Strings(names)
	return names, err
}

func isMockDir(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, "stubs")); err == nil && info.IsDir() {
		return true
	}
	protos, _ := filepath.Glob(filepath.Join(dir, "*.proto"))
	return len(protos) > 0
}
