// internal/container/cgroup/cgroup.go
package cgroup

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type Cgroup struct {
	Name        string
	File        *os.File
	CgroupRoot  string
	fileHandler FileHandler
}

type Factory interface {
	CreateCgroup(spec *Spec) (*Cgroup, error)
}

type DefaultFactory struct {
	subsystems  []Subsystem
	fileHandler FileHandler
}

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

type Subsystem interface {
	Name() string
	ApplySettings(cgroupPath string, resources *Resources) error
}

func NewCgroup(spec *Spec, subsystems []Subsystem, fileHandler FileHandler) (*Cgroup, error) {
	cgroupRoot := spec.CgroupRoot
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}
	cgroupPath := filepath.Join(cgroupRoot, spec.Name)
	if err := fileHandler.MkdirAll(cgroupPath, 0755); err != nil {
		zap.L().Error("Failed to create cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup directory %q: %v", cgroupPath, err)
	}

	tasksFilePath := filepath.Join(cgroupPath, "tasks")
	tasksFile, err := fileHandler.OpenFile(tasksFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		zap.L().Error("Failed to create tasks file for cgroup", zap.String("cgroupName", spec.Name), zap.Error(err))
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %v", spec.Name, err)
	}

	for _, subsystem := range subsystems {
		subsystemPath := filepath.Join(cgroupRoot, subsystem.Name(), spec.Name)

		if err := fileHandler.MkdirAll(subsystemPath, 0755); err != nil {
			zap.L().Error("Failed to create subsystem directory", zap.String("subsystemPath", subsystemPath), zap.Error(err))
			return nil, fmt.Errorf("failed to create subsystem directory %q: %v", subsystemPath, err)
		}

		if err := subsystem.ApplySettings(subsystemPath, spec.Resources); err != nil {
			zap.L().Error("Failed to apply subsystem settings", zap.String("subsystem", subsystem.Name()), zap.Error(err))
			return nil, err
		}
	}

	return &Cgroup{
		Name:        spec.Name,
		File:        tasksFile,
		CgroupRoot:  cgroupRoot,
		fileHandler: fileHandler,
	}, nil
}

func (cg *Cgroup) AddProcess(pid int) error {
	if _, err := fmt.Fprintf(cg.File, "%d\n", pid); err != nil {
		zap.L().Error("Failed to add process to cgroup", zap.Int("pid", pid), zap.String("cgroupName", cg.Name), zap.Error(err))
		return fmt.Errorf("failed to add process %d to cgroup %q: %w", pid, cg.Name, err)
	}
	return nil
}

func (cg *Cgroup) SetResourceLimits(resources *Resources) error {
	if resources.Memory != nil && resources.Memory.Limit > 0 {
		if err := cg.Set("memory.limit_in_bytes", fmt.Sprintf("%d", resources.Memory.Limit)); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
	}
	if resources.CPU != nil && resources.CPU.Shares > 0 {
		if err := cg.Set("cpu.shares", fmt.Sprintf("%d", resources.CPU.Shares)); err != nil {
			return fmt.Errorf("failed to set CPU shares: %w", err)
		}
	}
	if resources.BlkIO != nil && resources.BlkIO.Weight > 0 {
		if err := cg.Set("blkio.weight", fmt.Sprintf("%d", resources.BlkIO.Weight)); err != nil {
			return fmt.Errorf("failed to set Block I/O weight: %w", err)
		}
	}
	return nil
}

func (cg *Cgroup) Set(control string, value string) error {
	controlFile := filepath.Join(cg.CgroupRoot, cg.Name, control)
	f, err := cg.fileHandler.OpenFile(controlFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		zap.L().Error("Failed to open control file", zap.String("controlFile", controlFile), zap.Error(err))
		return fmt.Errorf("failed to open control file %s: %w", controlFile, err)
	}
	defer f.Close()
	if _, err := f.WriteString(value); err != nil {
		zap.L().Error("Failed to write value to control file", zap.String("controlFile", controlFile), zap.Error(err))
		return fmt.Errorf("failed to write value to control file %s: %w", controlFile, err)
	}
	return nil
}

func (cg *Cgroup) Close() error {
	if err := cg.File.Close(); err != nil {
		zap.L().Error("Failed to close cgroup file", zap.Error(err))
		return fmt.Errorf("failed to close cgroup file: %w", err)
	}
	return nil
}

func (cg *Cgroup) Remove() error {
	cgroupPath := filepath.Join(cg.CgroupRoot, cg.Name)
	if err := cg.fileHandler.RemoveAll(cgroupPath); err != nil {
		zap.L().Error("Failed to remove cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return fmt.Errorf("failed to remove cgroup directory %q: %w", cgroupPath, err)
	}
	return nil
}
