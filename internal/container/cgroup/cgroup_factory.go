package cgroup

import (
	"fmt"

	"go.uber.org/zap"
)

// Factory is an interface for creating Cgroup objects with different configurations based on the Spec provided.
type Factory interface {
	CreateCgroup(spec *Spec) (*Cgroup, error)
}

// DefaultFactory is a struct that implements the Factory interface and creates Cgroups using the specified subsystems.
type DefaultFactory struct {
	subsystems  []Subsystem
	fileHandler FileHandler
}

// NewDefaultFactory returns a new instance of DefaultFactory with the specified subsystems.
func NewDefaultFactory(subsystems []Subsystem, fileHandler FileHandler) *DefaultFactory {
	return &DefaultFactory{subsystems: subsystems, fileHandler: fileHandler}
}

func (f *DefaultFactory) CreateCgroup(spec *Spec) (*Cgroup, error) {
	cgroup, err := NewCgroup(spec, f.subsystems, f.fileHandler)
	if err != nil {
		zap.L().Error("failed to create cgroup", zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup: %v", err)
	}
	return cgroup, nil
}
