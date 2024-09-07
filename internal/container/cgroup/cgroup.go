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
		zap.L().Error("Failed to create cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return nil, fmt.Errorf("failed to create cgroup directory %q: %w", cgroupPath, err)
	}

	tasksFile, err := createTasksFile(cgroupPath, spec.Name, fileHandler)
	if err != nil {
		return nil, err
	}

	err = addSubsystems(cgroupRoot, subsystems, spec.Resources, fileHandler)
	if err != nil {
		return nil, err
	}

	return createCgroup(spec.Name, tasksFile, cgroupRoot, fileHandler), nil
}

func createTasksFile(cgroupPath, cgroupName string, fileHandler FileHandler) (*os.File, error) {
	tasksFilePath := filepath.Join(cgroupPath, "tasks")
	tasksFile, err := fileHandler.OpenFile(tasksFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		zap.L().Error("Failed to create tasks file for cgroup", zap.String("cgroupName", cgroupName), zap.Error(err))
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %w", cgroupName, err)
	}
	return tasksFile, nil
}

func addSubsystems(cgroupRoot string, subsystems []Subsystem, resources *Resources, fileHandler FileHandler) error {
	for _, subsystem := range subsystems {
		subsystemPath := filepath.Join(cgroupRoot, subsystem.Name())

		// Create subsystem directory if it doesn't exist
		if err := fileHandler.MkdirAll(subsystemPath, 0755); err != nil {
			zap.L().Error("Failed to create subsystem directory", zap.String("subsystemPath", subsystemPath), zap.Error(err))
			return fmt.Errorf("failed to create subsystem directory %q: %w", subsystemPath, err)
		}

		if err := subsystem.ApplySettings(subsystemPath, resources); err != nil {
			zap.L().Error("Failed to apply subsystem settings", zap.String("subsystemPath", subsystemPath), zap.Error(err))
			return fmt.Errorf("failed to apply settings for subsystem %q: %w", subsystem.Name(), err)
		}
	}
	return nil
}

func createCgroup(cgroupName string, tasksFile *os.File, cgroupRoot string, fileHandler FileHandler) *Cgroup {
	return &Cgroup{
		Name:        cgroupName,
		File:        tasksFile,
		CgroupRoot:  cgroupRoot,
		fileHandler: fileHandler,
	}
}

// Set sets the value of the specified control for the cgroup.
// This function takes a control (e.g. "memory.limit_in_bytes") and a value (e.g. "1024") as arguments,
// and writes the value to the control file.
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

// Close releases the cgroup's resources.
// This function closes the file descriptor for the cgroup's tasks file.
func (cg *Cgroup) Close() error {
	if err := cg.File.Close(); err != nil {
		zap.L().Error("Failed to close cgroup file", zap.Error(err))
		return fmt.Errorf("failed to close cgroup file: %w", err)
	}
	return nil
}

// Remove deletes the cgroup after closing its resources.
// This function removes the cgroup directory from the filesystem.
func (cg *Cgroup) Remove() error {
	cgroupPath := filepath.Join(cg.CgroupRoot, cg.Name)
	if err := cg.fileHandler.RemoveAll(cgroupPath); err != nil {
		zap.L().Error("Failed to remove cgroup directory", zap.String("cgroupPath", cgroupPath), zap.Error(err))
		return fmt.Errorf("failed to remove cgroup directory %q: %w", cgroupPath, err)
	}
	return nil
}

// AddProcess adds a process to the cgroup by writing the process ID to the tasks file.
func (cg *Cgroup) AddProcess(pid int) error {
	if _, err := fmt.Fprintf(cg.File, "%d\n", pid); err != nil {
		zap.L().Error("Failed to add process to cgroup", zap.Int("pid", pid), zap.String("cgroupName", cg.Name), zap.Error(err))
		return fmt.Errorf("failed to add process %d to cgroup %q: %w", pid, cg.Name, err)
	}
	return nil
}