package cgroup

import (
	"fmt"

	"go.uber.org/zap"
)

// Factory is an interface for creating Cgroup objects with different configurations based on the Spec provided.
type Factory interface {
	CreateCgroup(spec *Spec) (*Cgroup, error)
}

// DefaultCgroupFactory is a struct that implements the CgroupFactory interface and creates Cgroups using the specified subsystems.
type DefaultCgroupFactory struct {
	subsystems  []Subsystem
	fileHandler FileHandler
}

// NewDefaultCgroupFactory returns a new instance of DefaultCgroupFactory with the specified subsystems.
func NewDefaultCgroupFactory(subsystems []Subsystem, fileHandler FileHandler) *DefaultCgroupFactory {
	return &DefaultCgroupFactory{subsystems: subsystems, fileHandler: fileHandler}
}

func (f *DefaultCgroupFactory) CreateCgroup(spec *Spec) (*Cgroup, error) {
	cgroup, err := NewCgroup(spec, f.subsystems, f.fileHandler)
	if err != nil {
		zap.L().Error("failed to create cgroup", zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup: %v", err)
	}
	return cgroup, nil
}
