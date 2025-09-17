package loader

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
)

func isProtoFile(name string) bool {
	return strings.HasSuffix(name, ".proto")
}

func parseProtoAndStubs(protoPath string) ([]models.Route, error) {
	var routes []models.Route

	dir := filepath.Dir(protoPath)
	parser := protoparse.Parser{ImportPaths: []string{dir}}
	fds, err := parser.ParseFiles(filepath.Base(protoPath))
	if err != nil || len(fds) == 0 {
		log.Printf("Failed to parse proto file %s: %v", protoPath, err)
		return nil, err
	}

	stubDir := filepath.Join(dir, "stubs")
	stubFiles, err := os.ReadDir(stubDir)
	if err != nil {
		log.Printf("No stub folder found in %s: %v", dir, err)
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

		if s.Response.Message == "" || s.Response.Status == 0 || s.Request.URL == "" {
			log.Printf("Missing required fields in stub %s", stubPath)
			continue
		}

		var msg *dynamic.Message
		var rawJSONBody []byte

		if s.Response.Proto {
			msgDesc := findMessage(fds, s.Response.Message)
			if msgDesc == nil {
				log.Printf("Message type %s not found in proto file %s", s.Response.Message, protoPath)
				continue
			}

			msg = dynamic.NewMessage(msgDesc)
			if err := msg.UnmarshalJSON(s.Response.Body); err != nil {
				log.Printf("Failed to unmarshal response for %s: %v", stubPath, err)
				continue
			}
		} else {
			rawJSONBody = s.Response.Body
		}

		routes = append(routes, models.Route{
			URL:          s.Request.URL,
			Method:       s.Request.Method,
			HeaderMatch:  s.Request.Headers,
			BodyMatch:    s.Request.Body,
			Status:       s.Response.Status,
			Message:      msg,
			ProtoEncoded: s.Response.Proto,
			RawJSONBody:  rawJSONBody,
		})
	}

	return routes, nil
}
