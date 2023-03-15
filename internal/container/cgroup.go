// Package container provides functions for creating a container.
package container

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// NewCgroup returns a new cgroup object.
func NewCgroup(spec *CgroupSpec) (*Cgroup, error) {
	subsystems := []string{"cpu", "memory"}            // List of subsystems to use for the cgroup
	cgroupRoot := "/sys/fs/cgroup"                     // Root directory for cgroups
	cgroupPath := filepath.Join(cgroupRoot, spec.Name) // Path to the cgroup directory

	// Create the cgroup directory if it doesn't exist
	if err := os.MkdirAll(cgroupPath, 0777); err != nil {
		return nil, fmt.Errorf("failed to create cgroup directory %q: %v", cgroupPath, err)
	}

	// Create a file that tracks the cgroup tasks
	tasksFile, err := os.Create(filepath.Join(cgroupPath, "tasks"))
	if err != nil {
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %v", spec.Name, err)
	}
	errClose := tasksFile.Close()
	if errClose != nil {
		return nil, fmt.Errorf("failed to close tasks file for cgroup %q: %v", spec.Name, errClose)
	}

	// Add the current process to the cgroup tasks file
	pid := os.Getpid()
	if _, err := tasksFile.WriteString(fmt.Sprintf("%d", pid)); err != nil {
		return nil, fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, spec.Name, err)
	}

	// Open files for each subsystem and set the cgroup value to the cgroup directory
	subsystemFiles := make([]*os.File, 0, len(subsystems))
	for _, subsystem := range subsystems {
		subsystemPath := filepath.Join(cgroupRoot, subsystem, spec.Name)
		subsystemFile, err := os.OpenFile(filepath.Join(subsystemPath, subsystem+".max"), os.O_WRONLY, 0644)
		if err != nil {
			// Check if the error is due to the subsystem file not existing
			if os.IsNotExist(err) {
				continue // Skip this subsystem and move on to the next one
			}
			return nil, fmt.Errorf("failed to open %s file for cgroup %q: %v", subsystem, spec.Name, err)
		}

		if subsystem == "memory" {
			// Set the memory limit for the cgroup
			if _, err := subsystemFile.WriteString(fmt.Sprintf("%d", spec.Resources.Memory.Limit)); err != nil {
				return nil, fmt.Errorf("failed to set %s value for cgroup %q: %v", subsystem+".max", spec.Name, err)
			}
		} else {
			// Set the maximum value for the control to -1 (unlimited)
			if _, err := subsystemFile.WriteString("-1"); err != nil {
				return nil, fmt.Errorf("failed to set %s value for cgroup %q: %v", subsystem+".max", spec.Name, err)
			}
		}

		subsystemFiles = append(subsystemFiles, subsystemFile)
	}

	// Return a new cgroup object
	return &Cgroup{
		Name: spec.Name,
		File: tasksFile,
	}, nil
}

// Cgroup is an abstraction over a Linux control group.
type Cgroup struct {
	Name string
	File *os.File
}

// Set sets the value of the specified control.
func (cg *Cgroup) Set(control string, value string) error {
	const cgroupPath = "/sys/fs/cgroup"
	controlFile := filepath.Join(cgroupPath, cg.Name, control)
	f, err := os.OpenFile(controlFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open control file %s: %v", controlFile, err)
	}
	errClose := f.Close()
	if errClose != nil {
		return fmt.Errorf("failed to close tasks file for cgroup %q: %v", controlFile, errClose)
	}

	if _, err := f.WriteString(value); err != nil {
		return fmt.Errorf("failed to write value to control file %s: %v", controlFile, err)
	}

	return nil
}

// Close releases the cgroup's resources.
func (cg *Cgroup) Close() error {
	// Close the namespace file descriptor
	if err := cg.File.Close(); err != nil {
		return fmt.Errorf("failed to close cgroup file: %v", err)
	}

	return nil
}

// CgroupSpec represents the specification for a Linux control group.
type CgroupSpec struct {
	Name      string
	Resources *Resources
}

// The Resources struct contains a Memory struct that represents the memory resource allocation for a Linux control group.
type Resources struct {
	Memory *Memory
}

// Memory struct that represents the memory resource allocation for a Linux control group
type Memory struct {
	Limit int
}

// MustLimitMemory limits the memory usage of the current process.
func MustLimitMemory(maxMemory int64) {
	const memoryLimitControl = "memory.limit_in_bytes"
	cgroupSpec := &CgroupSpec{Name: "container"}
	cgroup, err := NewCgroup(cgroupSpec)
	if err != nil {
		log.Fatalf("failed to create cgroup: %v", err)
	}
	defer cgroup.Close()

	if err := cgroup.Set(memoryLimitControl, fmt.Sprintf("%d", maxMemory)); err != nil {
		log.Fatalf("failed to set %s for cgroup %s: %v", memoryLimitControl, cgroupSpec.Name, err)
	}
}
