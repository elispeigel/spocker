// Package container provides functions for creating a container, limiting its resources using Linux control groups (cgroups).
package container

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)


// NewCgroup returns a new cgroup object based on the given specification.
// The cgroup will be created with the specified name, and resources will be limited according to the given resource allocation.
func NewCgroup(spec *CgroupSpec) (*Cgroup, error) {
	subsystems := []string{"cpu", "memory"}            // List of subsystems to use for the cgroup
	cgroupRoot := "/sys/fs/cgroup"                     // Root directory for cgroups
	cgroupPath := filepath.Join(cgroupRoot, spec.Name) // Path to the cgroup directory

	// Create the cgroup directory if it doesn't exist
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cgroup directory %q: %v", cgroupPath, err)
	}

	// Create a file that tracks the cgroup tasks
	tasksFilePath := filepath.Join(cgroupPath, "tasks")
	tasksFile, err := os.OpenFile(tasksFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %v", spec.Name, err)
	}
	defer tasksFile.Close()

	// Add the current process to the cgroup tasks file
	pid := os.Getpid()
	if _, err := fmt.Fprintf(tasksFile, "%d\n", pid); err != nil {
		return nil, fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, spec.Name, err)
	}

	// Open files for each subsystem and set the cgroup value to the cgroup directory
    for _, subsystem := range subsystems {
        subsystemPath := filepath.Join(cgroupRoot, subsystem, spec.Name)

        // Set the initial values for the subsystem
        if subsystem == "cpu" {
            subsystemFile, err := os.OpenFile(filepath.Join(subsystemPath, "cpu.shares"), os.O_WRONLY, 0644)
            if err == nil {
                defer subsystemFile.Close()
                if _, err := fmt.Fprintf(subsystemFile, "%d", spec.Resources.CPU.Shares); err != nil {
                    return nil, fmt.Errorf("failed to set %s value for cgroup %q: %v", "cpu.shares", spec.Name, err)
                }
            }
        } else if subsystem == "memory" {
            subsystemFile, err := os.OpenFile(filepath.Join(subsystemPath, "memory.limit_in_bytes"), os.O_WRONLY, 0644)
            if err == nil {
                defer subsystemFile.Close()
                if _, err := fmt.Fprintf(subsystemFile, "%d", spec.Resources.Memory.Limit); err != nil {
                    return nil, fmt.Errorf("failed to set %s value for cgroup %q: %v", "memory.limit_in_bytes", spec.Name, err)
                }
            }
        } else if subsystem == "blkio" {
            subsystemFile, err := os.OpenFile(filepath.Join(subsystemPath, "blkio.weight"), os.O_WRONLY, 0644)
            if err == nil {
                defer subsystemFile.Close()
                if _, err := fmt.Fprintf(subsystemFile, "%d", spec.Resources.BlkIO.Weight); err != nil {
                    return nil, fmt.Errorf("failed to set %s value for cgroup %q: %v", "blkio.weight", spec.Name, err)
                }
            }
        }
    }

    // Return a new cgroup object
    return &Cgroup{
        Name:       spec.Name,
        File:       tasksFile,
        CgroupRoot: spec.CgroupRoot, // Add this field
    }, nil
}

// Cgroup is an abstraction over a Linux control group.
// It contains the name of the cgroup, a file descriptor for the tasks file, and the root path to the cgroup.
type Cgroup struct {
    Name       string
    File       *os.File
    CgroupRoot string
}

// Set sets the value of the specified control for the cgroup.
// This function takes a control (e.g. "memory.limit_in_bytes") and a value (e.g. "1024") as arguments,
// and writes the value to the control file.
func (cg *Cgroup) Set(control string, value string) error {
	const cgroupPath = "/sys/fs/cgroup"
	controlFile := filepath.Join(cgroupPath, cg.Name, control)
	f, err := os.OpenFile(controlFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open control file %s: %v", controlFile, err)
	}
	defer f.Close()
	if _, err := f.WriteString(value); err != nil {
		return fmt.Errorf("failed to write value to control file %s: %v", controlFile, err)
	}

	return nil
}

// Close releases the cgroup's resources.
// This function closes the file descriptor for the cgroup's tasks file.
func (cg *Cgroup) Close() error {
	// Close the namespace file descriptor
	if err := cg.File.Close(); err != nil {
		return fmt.Errorf("failed to close cgroup file: %v", err)
	}
	return nil
}

// Remove deletes the cgroup after closing its resources.
// This function removes the cgroup directory from the filesystem.
func (cg *Cgroup) Remove() error {
    cgroupPath := filepath.Join(cg.CgroupRoot, cg.Name) // Use the CgroupRoot field
    if err := os.RemoveAll(cgroupPath); err != nil {
        return fmt.Errorf("failed to remove cgroup directory %q: %v", cgroupPath, err)
    }
    return nil
}


// CgroupSpec represents the specification for a Linux control group.
// It contains the name of the cgroup, resources to be allocated, and the root path to the cgroup.
type CgroupSpec struct {
	Name          string
	Resources     *Resources
	CgroupRoot    string
}


// Resources struct contains the resource allocations for a Linux control group.
// It has fields for memory, CPU, and block I/O resources.
type Resources struct {
	Memory *Memory
	CPU    *CPU
	BlkIO  *BlkIO
}

// CPU struct represents the CPU resource allocation for a Linux control group.
// It contains a field for CPU shares.
type CPU struct {
	Shares int
}

// BlkIO struct represents the block I/O resource allocation for a Linux control group.
// It contains a field for block I/O weight.
type BlkIO struct {
	Weight int
}

// Memory struct represents the memory resource allocation for a Linux control group.
// It contains a field for memory limit.
type Memory struct {
	Limit int
}

// MustLimitMemory limits the memory usage of the current process.
// This function takes a maximum memory value (in bytes) as an argument and limits the memory usage of the current process accordingly.
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
