// Package store creates, updates and deletes mock files inside a mocks folder.
//
// Every name coming from the UI is validated so a request can never touch files
// outside the mocks folder, and every write is atomic (temp file + rename) so the
// hot-reload watcher never sees a half-written file.
package store

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/avijeet7/protomock/internal/loader"
)

// ErrInvalidName is returned for kinds, mock names or file names that aren't allowed.
var ErrInvalidName = errors.New("invalid name")

var (
	segmentRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	stubFileRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.json$`)
	protoFileRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.proto$`)
)

// Store edits files under Root.
type Store struct {
	Root string
}

// MockDir returns the folder of a mock, validating the kind and name.
// Mock names may be nested ("team/users"); each segment must be a plain name.
func (s *Store) MockDir(kind loader.Kind, mock string) (string, error) {
	if kind != loader.KindHTTP && kind != loader.KindGRPC {
		return "", fmt.Errorf("%w: kind must be http or grpc", ErrInvalidName)
	}
	if mock == "" {
		return "", fmt.Errorf("%w: mock name is required", ErrInvalidName)
	}
	for _, seg := range strings.Split(mock, "/") {
		if !segmentRE.MatchString(seg) {
			return "", fmt.Errorf("%w: mock name %q: use letters, digits, '.', '_' or '-' (nested folders with '/')", ErrInvalidName, mock)
		}
	}
	return filepath.Join(s.Root, string(kind), filepath.FromSlash(mock)), nil
}

// StubPath returns the path of a stub file, validating all names.
func (s *Store) StubPath(kind loader.Kind, mock, file string) (string, error) {
	dir, err := s.MockDir(kind, mock)
	if err != nil {
		return "", err
	}
	if !stubFileRE.MatchString(file) {
		return "", fmt.Errorf("%w: stub file %q must be a plain name ending in .json", ErrInvalidName, file)
	}
	return filepath.Join(dir, "stubs", file), nil
}

// ProtoPath returns the path of a .proto file in a mock folder, validating all names.
func (s *Store) ProtoPath(kind loader.Kind, mock, file string) (string, error) {
	dir, err := s.MockDir(kind, mock)
	if err != nil {
		return "", err
	}
	if !protoFileRE.MatchString(file) {
		return "", fmt.Errorf("%w: proto file %q must be a plain name ending in .proto", ErrInvalidName, file)
	}
	return filepath.Join(dir, file), nil
}

// CreateMock creates an empty mock folder (with its stubs/ folder).
func (s *Store) CreateMock(kind loader.Kind, mock string) error {
	dir, err := s.MockDir(kind, mock)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: mock %s/%s already exists", fs.ErrExist, kind, mock)
	}
	return os.MkdirAll(filepath.Join(dir, "stubs"), 0o755)
}

// WriteFile atomically writes content to path (creating parent folders).
func WriteFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".protomock-*.tmp")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op after a successful rename

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Delete removes a stub or proto file. Folders are never removed.
func Delete(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a folder", path)
	}
	return os.Remove(path)
}

// Writable reports whether files can be created in the mocks folder
// (false e.g. when it is mounted read-only in Docker).
func (s *Store) Writable() bool {
	dir := s.Root
	if _, err := os.Stat(dir); err != nil {
		dir = filepath.Dir(dir) // the mocks folder can be created on first write
	}
	f, err := os.CreateTemp(dir, ".protomock-write-check-*")
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return true
}
