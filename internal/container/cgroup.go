// Package container provides a set of utilities to manage Linux control groups (cgroups).
// This package allows the creation and management of cgroups, applying resource limits,
// and running processes within those cgroups.
package container

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// CgroupFactory is an interface for creating Cgroup objects with different configurations based on the CgroupSpec provided.
type CgroupFactory interface {
	CreateCgroup(spec *CgroupSpec) (*Cgroup, error)
}

// DefaultCgroupFactory is a struct that implements the CgroupFactory interface and creates Cgroups using the specified subsystems.
type DefaultCgroupFactory struct {
	subsystems []Subsystem
}

// NewDefaultCgroupFactory returns a new instance of DefaultCgroupFactory with the specified subsystems.
func NewDefaultCgroupFactory(subsystems []Subsystem) *DefaultCgroupFactory {
	return &DefaultCgroupFactory{subsystems: subsystems}
}

func (f *DefaultCgroupFactory) CreateCgroup(spec *CgroupSpec) (*Cgroup, error) {
	cgroup, err := NewCgroup(spec, f.subsystems)
	if err != nil {
		return nil, fmt.Errorf("failed to create cgroup: %v", err)
	}
	return cgroup, nil
}

// Subsystem represents a cgroup subsystem.
type Subsystem interface {
	Name() string
	ApplySettings(cgroupPath string, resources *Resources) error
}

// CPUSubsystem is an implementation of the Subsystem interface for the "cpu" subsystem.
type CPUSubsystem struct{}

func (c *CPUSubsystem) Name() string {
	return "cpu"
}

func (c *CPUSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(cgroupPath, "cpu.shares", resources.CPU.Shares)
}

// MemorySubsystem is an implementation of the Subsystem interface for the "memory" subsystem.
type MemorySubsystem struct{}

func (m *MemorySubsystem) Name() string {
	return "memory"
}

func (m *MemorySubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(cgroupPath, "memory.limit_in_bytes", resources.Memory.Limit)
}

// BlkIOSubsystem is an implementation of the Subsystem interface for the "blkio" subsystem.
type BlkIOSubsystem struct{}

func (b *BlkIOSubsystem) Name() string {
	return "blkio"
}

func (b *BlkIOSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(cgroupPath, "blkio.weight", resources.BlkIO.Weight)
}

// NewCgroup returns a new cgroup object based on the given specification.
// The cgroup will be created with the specified name, and resources will be limited according to the given resource allocation.
func NewCgroup(spec *CgroupSpec, subsystems []Subsystem) (*Cgroup, error) {
	cgroupRoot := spec.CgroupRoot
	if cgroupRoot == "" {
		cgroupRoot = "/sys/fs/cgroup"
	}
	cgroupPath := filepath.Join(cgroupRoot, spec.Name)
	if err := os.MkdirAll(cgroupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cgroup directory %q: %v", cgroupPath, err)
	}

	tasksFilePath := filepath.Join(cgroupPath, "tasks")
	tasksFile, err := os.OpenFile(tasksFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create tasks file for cgroup %q: %v", spec.Name, err)
	}
	defer tasksFile.Close()

	pid := os.Getpid()
	if _, err := fmt.Fprintf(tasksFile, "%d\n", pid); err != nil {
		return nil, fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, spec.Name, err)
	}

	for _, subsystem := range subsystems {
		subsystemPath := filepath.Join(cgroupRoot, subsystem.Name(), spec.Name)

		// Create subsystem directory if it doesn't exist
		if err := os.MkdirAll(subsystemPath, 0755); err != nil {
			return nil, fmt.Errorf("failed to create subsystem directory %q: %v", subsystemPath, err)
		}

		if err := subsystem.ApplySettings(subsystemPath, spec.Resources); err != nil {
			return nil, err
		}
	}

	return &Cgroup{
		Name:       spec.Name,
		File:       tasksFile,
		CgroupRoot: cgroupRoot,
	}, nil
}

func setSubsystemValue(subsystemPath, filename string, value int) error {
	subsystemFile, err := os.OpenFile(filepath.Join(subsystemPath, filename), os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open %s for cgroup: %v", filename, err)
	}
	defer subsystemFile.Close()
	if _, err := fmt.Fprintf(subsystemFile, "%d", value); err != nil {
		return fmt.Errorf("failed to set %s value for cgroup: %v", filename, err)
	}
	return nil
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
	controlFile := filepath.Join(cg.CgroupRoot, cg.Name, control)
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
	if err := cg.File.Close(); err != nil {
		return fmt.Errorf("failed to close cgroup file: %v", err)
	}
	return nil
}

// Remove deletes the cgroup after closing its resources.
// This function removes the cgroup directory from the filesystem.
func (cg *Cgroup) Remove() error {
	cgroupPath := filepath.Join(cg.CgroupRoot, cg.Name)
	if err := os.RemoveAll(cgroupPath); err != nil {
		return fmt.Errorf("failed to remove cgroup directory %q: %v", cgroupPath, err)
	}
	return nil
}

// CgroupSpecBuilder is a builder for CgroupSpec objects.
type CgroupSpecBuilder struct {
	spec *CgroupSpec
}

// NewCgroupSpecBuilder creates a new CgroupSpecBuilder.
func NewCgroupSpecBuilder() *CgroupSpecBuilder {
	return &CgroupSpecBuilder{
		spec: &CgroupSpec{},
	}
}

// WithName sets the name of the cgroup spec.
func (b *CgroupSpecBuilder) WithName(name string) *CgroupSpecBuilder {
	b.spec.Name = name
	return b
}

// WithResources sets the resources of the cgroup spec.
func (b *CgroupSpecBuilder) WithResources(resources *Resources) *CgroupSpecBuilder {
	b.spec.Resources = resources
	return b
}

// WithCgroupRoot sets the cgroup root of the cgroup spec.
func (b *CgroupSpecBuilder) WithCgroupRoot(cgroupRoot string) *CgroupSpecBuilder {
	b.spec.CgroupRoot = cgroupRoot
	return b
}

// Build constructs the CgroupSpec object using the provided settings.
func (b *CgroupSpecBuilder) Build() *CgroupSpec {
	return b.spec
}

// CgroupSpec represents the specification for a Linux control group.
// It contains the name of the cgroup, resources to be allocated, and the root path to the cgroup.
type CgroupSpec struct {
	Name       string
	Resources  *Resources
	CgroupRoot string
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
	cgroupSpec := NewCgroupSpecBuilder().
		WithName("container").
		Build()
	subsystems := []Subsystem{&CPUSubsystem{}, &MemorySubsystem{}, &BlkIOSubsystem{}}
	factory := NewDefaultCgroupFactory(subsystems)
	cgroup, err := factory.CreateCgroup(cgroupSpec)

	if err != nil {
		log.Fatalf("failed to create cgroup: %v", err)
	}
	defer cgroup.Close()
	if err := cgroup.Set(memoryLimitControl, fmt.Sprintf("%d", maxMemory)); err != nil {
		log.Fatalf("failed to set %s for cgroup %s: %v", memoryLimitControl, cgroupSpec.Name, err)
	}
}

// FindCgroupMountpoint returns the mountpoint of the cgroup hierarchy with the given subsystem.
func FindCgroupMountpoint(subsystem string) (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", fmt.Errorf("failed to open mountinfo: %v", err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Split(s.Text(), " ")
		if len(fields) < 5 {
			continue
		}

		options := strings.Split(fields[3], ",")
		for _, opt := range options {
			if opt == subsystem {
				return fields[4], nil
			}
		}
	}

	if err := s.Err(); err != nil {
		return "", fmt.Errorf("failed to scan mountinfo: %v", err)
	}

	return "", fmt.Errorf("cgroup subsystem %s not found", subsystem)
}

// ensureCgroupPathPrefix checks if the given path has the expected cgroup path prefix.
func ensureCgroupPathPrefix(cgroupPath string) error {
	if !strings.HasPrefix(cgroupPath, "/sys/fs/cgroup/") {
		return fmt.Errorf("invalid cgroup path: %s", cgroupPath)
	}
	return nil
}

// GetCgroupParam returns the value of the given cgroup parameter.
func GetCgroupParam(cgroupPath string, param string) (string, error) {
	if err := ensureCgroupPathPrefix(cgroupPath); err != nil {
		return "", err
	}

	filePath := filepath.Join(cgroupPath, param)
	valueBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup parameter %s: %v", param, err)
	}
	return string(bytes.TrimSpace(valueBytes)), nil
}

// SetCgroupParam sets the value of the given cgroup parameter.
func SetCgroupParam(cgroupPath string, param string, value string) error {
	if err := ensureCgroupPathPrefix(cgroupPath); err != nil {
		return err
	}
	paramFile := filepath.Join(cgroupPath, param)
	file, err := os.OpenFile(paramFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open cgroup parameter file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(value)
	if err != nil {
		return fmt.Errorf("failed to write cgroup parameter value: %v", err)
	}

	return nil
}

// ExecContainer runs the container process inside its namespaces.
func ExecContainer(containerID string, command []string) error {
	// Set up namespaces
	ctx := context.Background()
	cmd, err := createCommand(ctx, command[0], command[1:]...)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up cgroup
	cgroupConfig := NewCgroupSpecBuilder().
		WithName(containerID).
		WithResources(&Resources{
			Memory: &Memory{
				Limit: 1024 * 1024 * 1024, // 1 GB
			},
		}).
		Build()
	subsystems := []Subsystem{&CPUSubsystem{}, &MemorySubsystem{}, &BlkIOSubsystem{}}
	factory := NewDefaultCgroupFactory(subsystems)
	cgroup, err := factory.CreateCgroup(cgroupConfig)

	if err != nil {
		return err
	}
	if err := cgroup.AddProcess(os.Getpid()); err != nil {
		return err
	}
	defer cgroup.Close()

	// Start the container process
	runErr := cmd.Run()

	if runErr != nil {
		return fmt.Errorf("failed to execute container process: %v", runErr)
	}

	return nil
}

// AddProcess adds a process to the cgroup by writing the process ID to the tasks file.
func (cg *Cgroup) AddProcess(pid int) error {
	tasksFilePath := filepath.Join(cg.CgroupRoot, cg.Name, "tasks")
	tasksFile, err := os.OpenFile(tasksFilePath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tasks file for cgroup %q: %v", cg.Name, err)
	}
	defer tasksFile.Close()

	if _, err := fmt.Fprintf(tasksFile, "%d\n", pid); err != nil {
		return fmt.Errorf("failed to add process %d to cgroup %q: %v", pid, cg.Name, err)
	}

	return nil
}
