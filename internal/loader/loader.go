package loader

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"

	"github.com/avijeet7/protomock/internal/models"
)

// LoadMocks walks the provided root directory and loads all valid proto+stub combinations.
func LoadMocks(root string) ([]models.Route, error) {
	var routes []models.Route

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}

		dir := filepath.Dir(path)
		parser := protoparse.Parser{ImportPaths: []string{dir}}
		fds, err := parser.ParseFiles(d.Name())
		if err != nil || len(fds) == 0 {
			log.Printf("Failed to parse proto file %s: %v", path, err)
			return nil
		}

		stubDir := filepath.Join(dir, "stubs")
		stubFiles, err := os.ReadDir(stubDir)
		if err != nil {
			log.Printf("No stub folder found in %s: %v", dir, err)
			return nil
		}

		for _, stub := range stubFiles {
			if stub.IsDir() || !strings.HasSuffix(stub.Name(), ".json") {
				continue
			}

			stubPath := filepath.Join(stubDir, stub.Name())
			content, err := os.ReadFile(stubPath)
			if err != nil {
				log.Printf("Failed to read stub %s: %v", stubPath, err)
				continue
			}

			var s models.Stub
			if err := json.Unmarshal(content, &s); err != nil {
				log.Printf("Invalid JSON in %s: %v", stubPath, err)
				continue
			}

			if s.URL == "" || s.Status == 0 || s.Message == "" {
				log.Printf("Missing required fields in stub %s", stubPath)
				continue
			}

			msgDesc := findMessage(fds, s.Message)
			if msgDesc == nil {
				log.Printf("Message type %s not found in proto file %s", s.Message, path)
				continue
			}

			msg := dynamic.NewMessage(msgDesc)
			if err := msg.UnmarshalJSON(s.Response); err != nil {
				log.Printf("Failed to unmarshal response for %s: %v", stubPath, err)
				continue
			}

			routes = append(routes, models.Route{
				URL:     s.URL,
				Status:  s.Status,
				Message: msg,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return routes, nil
}

func findMessage(fds []*desc.FileDescriptor, name string) *desc.MessageDescriptor {
	for _, fd := range fds {
		if msg := fd.FindMessage(name); msg != nil {
			return msg
		}
		for _, dep := range fd.GetDependencies() {
			if msg := dep.FindMessage(name); msg != nil {
				return msg
			}
		}
	}
	return nil
}
