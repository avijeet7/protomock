//go:build integration

package integration

import (
	"context"
	"slices"
	"testing"
	"time"

	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestGRPC_ReflectionListsMockedServices(t *testing.T) {
	conn := startGreeter(t, map[string]string{"bob.json": greeterStub("bob", "hi bob")})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// 1. ListServices must include the mocked service.
	if err := stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{},
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range resp.GetListServicesResponse().GetService() {
		names = append(names, s.GetName())
	}
	if !slices.Contains(names, "test.Greeter") {
		t.Fatalf("ListServices = %v, want it to include test.Greeter", names)
	}

	// 2. The service's .proto must be resolvable by symbol, as grpcurl does.
	if err := stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: "test.Greeter"},
	}); err != nil {
		t.Fatal(err)
	}
	resp, err = stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	files := resp.GetFileDescriptorResponse().GetFileDescriptorProto()
	if len(files) == 0 {
		t.Fatalf("FileContainingSymbol(test.Greeter) returned no files: %v", resp.GetErrorResponse())
	}
	var fd descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal(files[0], &fd); err != nil {
		t.Fatal(err)
	}
	if len(fd.GetService()) != 1 || fd.GetService()[0].GetName() != "Greeter" {
		t.Errorf("file %s services = %v, want [Greeter]", fd.GetName(), fd.GetService())
	}
}
