// cgroup package manages Linux control groups (cgroups) and provides functionality to apply resource limitations.
package cgroup

import (
	"fmt"

	"go.uber.org/zap"
)

// NewDefaultFactory returns a new instance of DefaultFactory with the specified subsystems.
func NewDefaultFactory(subsystems []Subsystem, fileHandler FileHandler) *DefaultFactory {
	return &DefaultFactory{subsystems: subsystems, fileHandler: fileHandler}
}

// CreateCgroup creates a new Cgroup instance based on the provided Spec, using the DefaultFactory's subsystems and fileHandler. Returns an error if the creation fails.
func (f *DefaultFactory) CreateCgroup(spec *Spec) (*Cgroup, error) {
	cgroup, err := NewCgroup(spec, f.subsystems, f.fileHandler)
	if err != nil {
		zap.L().Error("failed to create cgroup", zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup: %v", err)
	}
	return cgroup, nil
}
