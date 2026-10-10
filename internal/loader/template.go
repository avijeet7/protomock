package loader

import (
	"encoding/json"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// maxTemplateDepth stops expanding nested messages (and breaks recursive types).
const maxTemplateDepth = 4

// Template returns an example JSON body for md with every field set to a sample
// value, using .proto field names. The editor offers it as a starting point.
func Template(md protoreflect.MessageDescriptor) json.RawMessage {
	out, _ := json.MarshalIndent(messageTemplate(md, 0, map[protoreflect.FullName]bool{}), "", "  ")
	return out
}

func messageTemplate(md protoreflect.MessageDescriptor, depth int, seen map[protoreflect.FullName]bool) any {
	if v, ok := wellKnownTemplate(md); ok {
		return v
	}
	obj := map[string]any{}
	if depth >= maxTemplateDepth || seen[md.FullName()] {
		return obj
	}
	seen[md.FullName()] = true
	defer delete(seen, md.FullName())

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		f := fields.Get(i)
		// Only one member of a oneof may be set: use the first.
		if oneof := f.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() && oneof.Fields().Get(0) != f {
			continue
		}
		obj[string(f.Name())] = fieldTemplate(f, depth, seen)
	}
	return obj
}

func fieldTemplate(f protoreflect.FieldDescriptor, depth int, seen map[protoreflect.FullName]bool) any {
	switch {
	case f.IsMap():
		return map[string]any{"key": singularTemplate(f.MapValue(), depth, seen)}
	case f.IsList():
		return []any{singularTemplate(f, depth, seen)}
	default:
		return singularTemplate(f, depth, seen)
	}
}

func singularTemplate(f protoreflect.FieldDescriptor, depth int, seen map[protoreflect.FullName]bool) any {
	switch f.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return messageTemplate(f.Message(), depth+1, seen)
	case protoreflect.EnumKind:
		if values := f.Enum().Values(); values.Len() > 0 {
			return string(values.Get(0).Name())
		}
		return 0
	case protoreflect.BoolKind:
		return false
	case protoreflect.StringKind:
		return ""
	case protoreflect.BytesKind:
		return "" // base64
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "0" // proto3 JSON encodes 64-bit integers as strings
	default:
		return 0
	}
}

// wellKnownTemplate covers types whose JSON form isn't a plain object.
func wellKnownTemplate(md protoreflect.MessageDescriptor) (any, bool) {
	switch md.FullName() {
	case "google.protobuf.Timestamp":
		return "1970-01-01T00:00:00Z", true
	case "google.protobuf.Duration":
		return "0s", true
	case "google.protobuf.FieldMask":
		return "", true
	case "google.protobuf.Struct", "google.protobuf.Empty":
		return map[string]any{}, true
	case "google.protobuf.Value":
		return nil, true
	case "google.protobuf.ListValue":
		return []any{}, true
	case "google.protobuf.StringValue", "google.protobuf.BytesValue":
		return "", true
	case "google.protobuf.BoolValue":
		return false, true
	case "google.protobuf.Int64Value", "google.protobuf.UInt64Value":
		return "0", true
	case "google.protobuf.Int32Value", "google.protobuf.UInt32Value",
		"google.protobuf.FloatValue", "google.protobuf.DoubleValue":
		return 0, true
	}
	return nil, false
}
