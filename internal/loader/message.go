package loader

import (
	"github.com/jhump/protoreflect/desc"
)

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
