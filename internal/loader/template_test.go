package loader

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/dynamicpb"
)

const richProto = `syntax = "proto3";
package rich;
import "google/protobuf/timestamp.proto";

enum Status { STATUS_UNKNOWN = 0; ACTIVE = 1; }
message Tree { string name = 1; repeated Tree children = 2; }
message Vendor {
  string id = 1;
  int64 revenue = 2;
  bool open = 3;
  Status status = 4;
  repeated string tags = 5;
  map<string, int32> stock = 6;
  google.protobuf.Timestamp updated_at = 7;
  Tree tree = 8;
  oneof contact { string email = 9; string phone = 10; }
}
`

func TestTemplate_IsValidForItsMessage(t *testing.T) {
	root := writeTree(t, map[string]string{"rich.proto": richProto})
	protos, err := CompileProtos(root, []string{"rich.proto"})
	if err != nil {
		t.Fatal(err)
	}
	md := protos.Message("rich.Vendor")
	tmpl := Template(md)

	// The template must parse as the message it was generated for.
	msg := dynamicpb.NewMessage(md)
	if err := protojson.Unmarshal(tmpl, msg); err != nil {
		t.Fatalf("template does not parse as rich.Vendor: %v\n%s", err, tmpl)
	}
	for _, want := range []string{`"revenue": "0"`, `"status": "STATUS_UNKNOWN"`, `"updated_at": "1970-01-01T00:00:00Z"`, `"children": [`, `"email": ""`} {
		if !contains(string(tmpl), want) {
			t.Errorf("template missing %s:\n%s", want, tmpl)
		}
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func TestTemplate_OneofUsesFirstMember(t *testing.T) {
	root := writeTree(t, map[string]string{"rich.proto": richProto})
	protos, err := CompileProtos(root, []string{"rich.proto"})
	if err != nil {
		t.Fatal(err)
	}
	if tmpl := string(Template(protos.Message("rich.Vendor"))); strings.Contains(tmpl, `"phone"`) {
		t.Errorf("template sets two oneof members:\n%s", tmpl)
	}
}
