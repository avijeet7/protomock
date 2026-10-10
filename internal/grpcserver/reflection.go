package grpcserver

import (
	"log"
	"strings"
	"sync/atomic"

	"github.com/avijeet7/protomock/internal/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// reflectionSource serves gRPC server reflection for the mocked services, so tools like
// grpcurl can list and call them without a local copy of the .proto files.
// Mocked services are handled by UnknownServiceHandler, so the grpc.Server itself
// doesn't know about them; this source describes them from the loaded proto files.
type reflectionSource struct {
	server *grpc.Server
	snap   atomic.Pointer[reflectionSnapshot]
}

type reflectionSnapshot struct {
	files    *protoregistry.Files
	services map[string]grpc.ServiceInfo
}

func registerReflection(server *grpc.Server) *reflectionSource {
	src := &reflectionSource{server: server}
	src.update(nil)

	opts := reflection.ServerOptions{Services: src, DescriptorResolver: src}
	reflectionv1.RegisterServerReflectionServer(server, reflection.NewServerV1(opts))
	reflectionv1alpha.RegisterServerReflectionServer(server, reflection.NewServer(opts))
	return src
}

// update replaces the described services with those used by routes.
func (s *reflectionSource) update(routes []models.Route) {
	snap := &reflectionSnapshot{files: new(protoregistry.Files), services: map[string]grpc.ServiceInfo{}}

	for _, route := range routes {
		if route.RequestDesc == nil {
			continue
		}
		serviceName, _, _ := strings.Cut(strings.TrimPrefix(route.URL, "/"), "/")
		if _, done := snap.services[serviceName]; done {
			continue
		}
		file := route.RequestDesc.ParentFile()
		sd := file.Services().ByName(protoreflect.FullName(serviceName).Name())
		if sd == nil || string(sd.FullName()) != serviceName {
			continue
		}
		if err := registerFileTree(snap.files, file); err != nil {
			log.Printf("[gRPC] ⚠️ Reflection unavailable for %s: %v", serviceName, err)
			continue
		}

		info := grpc.ServiceInfo{Metadata: file.Path()}
		for i := 0; i < sd.Methods().Len(); i++ {
			m := sd.Methods().Get(i)
			info.Methods = append(info.Methods, grpc.MethodInfo{
				Name:           string(m.Name()),
				IsClientStream: m.IsStreamingClient(),
				IsServerStream: m.IsStreamingServer(),
			})
		}
		snap.services[serviceName] = info
	}
	s.snap.Store(snap)
}

// registerFileTree registers file after everything it imports. A path that is
// already registered (e.g. a well-known import shared by two mocks) is skipped.
func registerFileTree(files *protoregistry.Files, file protoreflect.FileDescriptor) error {
	if _, err := files.FindFileByPath(file.Path()); err == nil {
		return nil
	}
	imports := file.Imports()
	for i := 0; i < imports.Len(); i++ {
		if err := registerFileTree(files, imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	return files.RegisterFile(file)
}

// GetServiceInfo lists the real registered services (reflection itself) plus the mocked ones.
func (s *reflectionSource) GetServiceInfo() map[string]grpc.ServiceInfo {
	all := s.server.GetServiceInfo()
	for name, info := range s.snap.Load().services {
		all[name] = info
	}
	return all
}

func (s *reflectionSource) FindFileByPath(path string) (protoreflect.FileDescriptor, error) {
	if fd, err := s.snap.Load().files.FindFileByPath(path); err == nil {
		return fd, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(path)
}

func (s *reflectionSource) FindDescriptorByName(name protoreflect.FullName) (protoreflect.Descriptor, error) {
	if d, err := s.snap.Load().files.FindDescriptorByName(name); err == nil {
		return d, nil
	}
	return protoregistry.GlobalFiles.FindDescriptorByName(name)
}
