package loader

import (
	"context"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Protos is a mock's compiled .proto files. Lookups also search everything they
// import, e.g. google.protobuf.Empty or messages from a shared common.proto.
type Protos struct {
	Files []protoreflect.FileDescriptor // the mock's own .proto files
	reg   *protoregistry.Files          // Files plus their transitive imports
}

// CompileProtos compiles the named .proto files in dir. Imports resolve relative to dir,
// and well-known types (google/protobuf/*.proto) are built in.
func CompileProtos(dir string, names []string) (*Protos, error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: []string{dir},
		}),
	}
	files, err := compiler.Compile(context.Background(), names...)
	if err != nil {
		return nil, err
	}

	p := &Protos{reg: new(protoregistry.Files)}
	for _, f := range files {
		p.Files = append(p.Files, f)
		if err := registerFileTree(p.reg, f); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// registerFileTree registers file after everything it imports; already-registered paths are skipped.
func registerFileTree(reg *protoregistry.Files, file protoreflect.FileDescriptor) error {
	if _, err := reg.FindFileByPath(file.Path()); err == nil {
		return nil
	}
	imports := file.Imports()
	for i := 0; i < imports.Len(); i++ {
		if err := registerFileTree(reg, imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	return reg.RegisterFile(file)
}

// Message finds a message type by full name (e.g. "greeter.HelloReply"), or returns nil.
func (p *Protos) Message(name string) protoreflect.MessageDescriptor {
	md, _ := p.find(name).(protoreflect.MessageDescriptor)
	return md
}

// Method finds a gRPC method by service full name and method name, or returns nil.
func (p *Protos) Method(service, method string) protoreflect.MethodDescriptor {
	sd, ok := p.find(service).(protoreflect.ServiceDescriptor)
	if !ok {
		return nil
	}
	return sd.Methods().ByName(protoreflect.Name(method))
}

func (p *Protos) find(name string) protoreflect.Descriptor {
	if p == nil {
		return nil
	}
	d, err := p.reg.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil
	}
	return d
}
