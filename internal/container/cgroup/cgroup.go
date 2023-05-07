package cgroup

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// NewCgroup returns a new cgroup object based on the given specification.
// The cgroup will be created with the specified name, and resources will be limited according to the given resource allocation.
func NewCgroup(spec *Spec, subsystems []Subsystem, fileHandler FileHandler) (*Cgroup, error) {
	cgroupRoot := spec.CgroupRoot
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}
	cgroupPath := filepath.Join(cgroupRoot, spec.Name)
	if err := fileHandler.MkdirAll(cgroupPath, 0755); err != nil {
		zap.L().Error("failed to create cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup directory %q: %v", cgroupPath, err)
	}

	tasksFilePath := filepath.Join(cgroupPath, "tasks")
	tasksFile, err := fileHandler.OpenFile(tasksFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		zap.L().Error("failed to create tasks file for cgroup", zap.String("cgroupName", spec.Name), zap.Error(err))
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %v", spec.Name, err)
	}
	defer tasksFile.Close()

	pid := os.Getpid()
	if _, err := fmt.Fprintf(tasksFile, "%d\n", pid); err != nil {
		zap.L().Error("failed to add process to cgroup", zap.Int("pid", pid), zap.String("cgroupName", spec.Name), zap.Error(err))
		return nil, fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, spec.Name, err)
	}

	for _, subsystem := range subsystems {
		subsystemPath := filepath.Join(cgroupRoot, subsystem.Name(), spec.Name)

		// Create subsystem directory if it doesn't exist
		if err := fileHandler.MkdirAll(subsystemPath, 0755); err != nil {
			zap.L().Error("failed to create subsystem directory", zap.String("subsystemPath", subsystemPath), zap.Error(err))
			return nil, fmt.Errorf("failed to create subsystem directory %q: %v", subsystemPath, err)
		}

		if err := subsystem.ApplySettings(subsystemPath, spec.Resources); err != nil {
			zap.L().Error("failed to apply subsystem settings", zap.Error(err))
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

// Cgroup is an abstraction over a Linux control group.
// It contains the name of the cgroup, a file descriptor for the tasks file, and the root path to the cgroup.
type Cgroup struct {
	Name        string
	File        *os.File
	CgroupRoot  string
	fileHandler FileHandler
}

// Set sets the value of the specified control for the cgroup.
// This function takes a control (e.g. "memory.limit_in_bytes") and a value (e.g. "1024") as arguments,
// and writes the value to the control file.
func (cg *Cgroup) Set(control string, value string) error {
	controlFile := filepath.Join(cg.CgroupRoot, cg.Name, control)
	f, err := cg.fileHandler.OpenFile(controlFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		zap.L().Error("failed to open control file", zap.String("controlFile", controlFile), zap.Error(err))
		return fmt.Errorf("failed to open control file %s: %v", controlFile, err)
	}
	defer f.Close()
	if _, err := f.WriteString(value); err != nil {
		zap.L().Error("failed to write value to control file", zap.String("controlFile", controlFile), zap.Error(err))
		return fmt.Errorf("failed to write value to control file %s: %v", controlFile, err)
	}
	return nil
}

// Close releases the cgroup's resources.
// This function closes the file descriptor for the cgroup's tasks file.
func (cg *Cgroup) Close() error {
	if err := cg.File.Close(); err != nil {
		zap.L().Error("failed toclose cgroup file", zap.Error(err))
		return fmt.Errorf("failed to close cgroup file: %v", err)
	}
	return nil
}

// Remove deletes the cgroup after closing its resources.
// This function removes the cgroup directory from the filesystem.
func (cg *Cgroup) Remove() error {
	cgroupPath := filepath.Join(cg.CgroupRoot, cg.Name)
	if err := cg.fileHandler.RemoveAll(cgroupPath); err != nil {
		zap.L().Error("failed to remove cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return fmt.Errorf("failed to remove cgroup directory %q: %v", cgroupPath, err)
	}
	return nil
}

// AddProcess adds a process to the cgroup by writing the process ID to the tasks file.
func (cg *Cgroup) AddProcess(pid int, fileHandler FileHandler) error {
	tasksFilePath := filepath.Join(cg.CgroupRoot, cg.Name, "tasks")
	tasksFile, err := fileHandler.OpenFile(tasksFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tasks file for cgroup %q: %v", cg.Name, err)
	}
	defer tasksFile.Close()

	if _, err := fmt.Fprintf(tasksFile, "%d\n", pid); err != nil {
		return fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, cg.Name, err)
	}

	return nil
}
