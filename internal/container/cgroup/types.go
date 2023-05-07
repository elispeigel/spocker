// cgroup package manages Linux control groups (cgroups) and provides functionality to apply resource limitations.
package cgroup

import "os"

type FileHandler interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	ReadFile(filename string) ([]byte, error)
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
}

type DefaultFileHandler struct{}

// Subsystem represents a cgroup subsystem.
type Subsystem interface {
	Name() string
	ApplySettings(cgroupPath string, resources *Resources) error
}

// CPUSubsystem is an implementation of the Subsystem interface for the "cpu" subsystem.
type CPUSubsystem struct {
	fileHandler FileHandler
}

// MemorySubsystem is an implementation of the Subsystem interface for the "memory" subsystem.
type MemorySubsystem struct {
	fileHandler FileHandler
}

// BlkIOSubsystem is an implementation of the Subsystem interface for the "blkio" subsystem.
type BlkIOSubsystem struct {
	fileHandler FileHandler
}

// Cgroup is an abstraction over a Linux control group.
// It contains the name of the cgroup, a file descriptor for the tasks file, and the root path to the cgroup.
type Cgroup struct {
	Name        string
	File        *os.File
	CgroupRoot  string
	fileHandler FileHandler
}

// Factory is an interface for creating Cgroup objects with different configurations based on the Spec provided.
type Factory interface {
	CreateCgroup(spec *Spec) (*Cgroup, error)
}

// DefaultFactory is a struct that implements the Factory interface and creates Cgroups using the specified subsystems.
type DefaultFactory struct {
	subsystems  []Subsystem
	fileHandler FileHandler
}
