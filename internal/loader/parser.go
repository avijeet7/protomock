package loader

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/avijeet7/protomock/internal/models"
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func isProtoFile(name string) bool {
	return strings.HasSuffix(name, ".proto")
}

// compileProto compiles a .proto file; imports resolve relative to its folder,
// and well-known types (google/protobuf/*.proto) are built in.
func compileProto(protoPath string) (linker.Files, error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{filepath.Dir(protoPath)},
		}),
	}
	return compiler.Compile(context.Background(), filepath.Base(protoPath))
}

func findMessage(files linker.Files, msgName string) protoreflect.MessageDescriptor {
	d, err := files.AsResolver().FindDescriptorByName(protoreflect.FullName(msgName))
	if err != nil {
		return nil
	}
	md, _ := d.(protoreflect.MessageDescriptor)
	return md
}

// findMethodInput returns the request message type for a gRPC path like
// "/pkg.Service/Method", or nil if the path doesn't name a method in the proto.
func findMethodInput(files linker.Files, url string) protoreflect.MessageDescriptor {
	service, method, ok := strings.Cut(strings.TrimPrefix(url, "/"), "/")
	if !ok {
		return nil
	}
	d, err := files.AsResolver().FindDescriptorByName(protoreflect.FullName(service))
	if err != nil {
		return nil
	}
	sd, ok := d.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil
	}
	if md := sd.Methods().ByName(protoreflect.Name(method)); md != nil {
		return md.Input()
	}
	return nil
}

// newMessage builds a dynamic message of type md from JSON (an empty body means an empty message).
func newMessage(md protoreflect.MessageDescriptor, body json.RawMessage) (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(md)
	if len(body) == 0 {
		return msg, nil
	}
	return msg, protojson.Unmarshal(body, msg)
}

func parseProtoAndStubs(protoPath string) ([]models.Route, error) {
	var routes []models.Route

	dir := filepath.Dir(protoPath)
	fds, err := compileProto(protoPath)
	if err != nil {
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

		if !s.Response.Proto {
			continue
		}

		if s.Response.Message == "" || s.Response.Status == 0 || s.Request.URL == "" {
			log.Printf("Missing required fields in stub %s", stubPath)
			continue
		}

		var msg *dynamicpb.Message
		var rawJSONBody []byte
		var msgDesc protoreflect.MessageDescriptor

		if s.Response.Proto {
			msgDesc = findMessage(fds, s.Response.Message)
			if msgDesc == nil {
				log.Printf("Message type %s not found in proto file %s", s.Response.Message, protoPath)
				continue
			}

			msg, err = newMessage(msgDesc, s.Response.Body)
			if err != nil {
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
			MessageDesc:  msgDesc,
			RequestDesc:  findMethodInput(fds, s.Request.URL),
			ProtoEncoded: s.Response.Proto,
			RawJSONBody:  rawJSONBody,
			FilePath:     stubPath,
		})
	}

	return routes, nil
}
