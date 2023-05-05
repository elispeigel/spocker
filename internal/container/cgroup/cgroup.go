package cgroup

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

	"go.uber.org/zap"

	"spocker/internal/container/util"
)

type FileHandler interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	ReadFile(filename string) ([]byte, error)
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
}

type DefaultFileHandler struct{}

func (d *DefaultFileHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (d *DefaultFileHandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (d *DefaultFileHandler) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (d *DefaultFileHandler) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

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

// Subsystem represents a cgroup subsystem.
type Subsystem interface {
	Name() string
	ApplySettings(cgroupPath string, resources *Resources) error
}

// CPUSubsystem is an implementation of the Subsystem interface for the "cpu" subsystem.
type CPUSubsystem struct {
	fileHandler FileHandler
}

func NewCPUSubsystem(fileHandler FileHandler) *CPUSubsystem {
	return &CPUSubsystem{fileHandler: fileHandler}
}

func (c *CPUSubsystem) Name() string {
	return "cpu"
}

func (c *CPUSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(c.fileHandler, cgroupPath, "cpu.shares", resources.CPU.Shares)
}

// MemorySubsystem is an implementation of the Subsystem interface for the "memory" subsystem.
type MemorySubsystem struct {
	fileHandler FileHandler
}

func NewMemorySubsystem(fileHandler FileHandler) *MemorySubsystem {
	return &MemorySubsystem{fileHandler: fileHandler}
}

func (m *MemorySubsystem) Name() string {
	return "memory"
}

func (m *MemorySubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(m.fileHandler, cgroupPath, "memory.limit_in_bytes", resources.Memory.Limit)
}

// BlkIOSubsystem is an implementation of the Subsystem interface for the "blkio" subsystem.
type BlkIOSubsystem struct {
	fileHandler FileHandler
}

func NewBlkIOSubsystem(fileHandler FileHandler) *BlkIOSubsystem {
	return &BlkIOSubsystem{fileHandler: fileHandler}
}

func (b *BlkIOSubsystem) Name() string {
	return "blkio"
}

func (b *BlkIOSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(b.fileHandler, cgroupPath, "blkio.weight", resources.BlkIO.Weight)
}

// NewCgroup returns a new cgroup object based on the given specification.
// The cgroup will be created with the specified name, and resources will be limited according to the given resource allocation.
func NewCgroup(spec *CgroupSpec, subsystems []Subsystem, fileHandler FileHandler) (*Cgroup, error) {
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

func setSubsystemValue(fileHandler FileHandler, subsystemPath, filename string, value int) error {
	subsystemFile, err := fileHandler.OpenFile(filepath.Join(subsystemPath, filename), os.O_WRONLY, 0644)
	if err != nil {
		zap.L().Error("failed to open cgroup subsystem file", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to open %s for cgroup: %v", filename, err)
	}
	defer subsystemFile.Close()
	if _, err := fmt.Fprintf(subsystemFile, "%d", value); err != nil {
		zap.L().Error("failed to set cgroup subsystem value", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to set %s value for cgroup: %v", filename, err)
	}
	return nil
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
		WithResources(&Resources{
			Memory: &Memory{
				Limit: int(maxMemory),
			},
		}).
		Build()
	fileHandler := &DefaultFileHandler{}
	subsystems := []Subsystem{
		NewCPUSubsystem(fileHandler),
		NewMemorySubsystem(fileHandler),
		NewBlkIOSubsystem(fileHandler),
	}
	factory := NewDefaultCgroupFactory(subsystems, fileHandler)
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
func FindCgroupMountpoint(subsystem string, fileHandler FileHandler) (string, error) {
	f, err := fileHandler.OpenFile("/proc/self/mountinfo", os.O_RDONLY, 0)
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
func GetCgroupParam(cgroupPath string, param string, fileHandler FileHandler) (string, error) {
	if err := ensureCgroupPathPrefix(cgroupPath); err != nil {
		return "", err
	}

	filePath := filepath.Join(cgroupPath, param)
	valueBytes, err := fileHandler.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup parameter %s: %v", param, err)
	}
	return string(bytes.TrimSpace(valueBytes)), nil
}

// SetCgroupParam sets the value of the given cgroup parameter.
func SetCgroupParam(cgroupPath string, param string, value string, fileHandler FileHandler) error {
	if err := ensureCgroupPathPrefix(cgroupPath); err != nil {
		return err
	}
	paramFile := filepath.Join(cgroupPath, param)
	file, err := fileHandler.OpenFile(paramFile, os.O_WRONLY|os.O_TRUNC, 0644)
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
	cmd, err := util.CreateCommand(ctx, command[0], command[1:]...)
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
	fileHandler := &DefaultFileHandler{}
	subsystems := []Subsystem{
		NewCPUSubsystem(fileHandler),
		NewMemorySubsystem(fileHandler),
		NewBlkIOSubsystem(fileHandler),
	}
	factory := NewDefaultCgroupFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(cgroupConfig)

	if err != nil {
		return err
	}
	if err := cgroup.AddProcess(os.Getpid(), fileHandler); err != nil {
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
