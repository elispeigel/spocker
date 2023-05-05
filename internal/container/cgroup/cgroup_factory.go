package cgroup

import (
	"fmt"

	"go.uber.org/zap"
)

// CgroupFactory is an interface for creating Cgroup objects with different configurations based on the CgroupSpec provided.
type CgroupFactory interface {
	CreateCgroup(spec *CgroupSpec) (*Cgroup, error)
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

func (f *DefaultCgroupFactory) CreateCgroup(spec *CgroupSpec) (*Cgroup, error) {
	cgroup, err := NewCgroup(spec, f.subsystems, f.fileHandler)
	if err != nil {
		zap.L().Error("failed to create cgroup", zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup: %v", err)
	}
	return cgroup, nil
}