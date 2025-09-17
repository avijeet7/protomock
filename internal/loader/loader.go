package loader

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
)

// LoadProtoMocks walks the provided root directory and loads all valid proto+stub combinations.
func LoadProtoMocks(root string) ([]models.Route, error) {
	var routes []models.Route

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isProtoFile(d.Name()) {
			return err
		}
		newRoutes, err := parseProtoAndStubs(path)
		if err == nil {
			routes = append(routes, newRoutes...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return routes, nil
}

// parseStandaloneJSONStubs parses JSON stub files from a given directory.
func parseStandaloneJSONStubs(stubDir string) ([]models.Route, error) {
	var routes []models.Route

	stubFiles, err := os.ReadDir(stubDir)
	if err != nil {
		return nil, err
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

		// For standalone JSON stubs, ensure ProtoEncoded is false and RawJSONBody is populated
		routes = append(routes, models.Route{
			URL:          s.Request.URL,
			Method:       s.Request.Method,
			HeaderMatch:  s.Request.Headers,
			BodyMatch:    s.Request.Body,
			Status:       s.Response.Status,
			Message:      nil, // No protobuf message for standalone JSON
			ProtoEncoded: false,
			RawJSONBody:  s.Response.Body,
		})
	}
	return routes, nil
}

// LoadJSONMocks walks the provided root directory and loads all standalone JSON stub combinations.
func LoadJSONMocks(root string) ([]models.Route, error) {
	var routes []models.Route

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "stubs" {
			// Check if this "stubs" directory is not a child of a proto file's directory
			// This is a heuristic: if the parent directory does not contain a .proto file,
			// then this "stubs" directory is considered standalone.
			parentDir := filepath.Dir(path)
			hasProto := false
			parentEntries, _ := os.ReadDir(parentDir)
			for _, entry := range parentEntries {
				if !entry.IsDir() && isProtoFile(entry.Name()) {
					hasProto = true
					break
				}
			}

			if !hasProto {
				newRoutes, err := parseStandaloneJSONStubs(path)
				if err == nil {
					routes = append(routes, newRoutes...)
				}
				return filepath.SkipDir // Skip subdirectories of this stubs directory
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return routes, nil
}
