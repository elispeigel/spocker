// Package container provides functions for creating a container, limiting its resources using Linux control groups (cgroups).
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

// NewCgroup returns a new cgroup object based on the given specification.
// The cgroup will be created with the specified name, and resources will be limited according to the given resource allocation.
func NewCgroup(spec *CgroupSpec) (*Cgroup, error) {
	subsystems := []string{"cpu", "memory", "blkio"} // Add "blkio" to the list of subsystems
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
		subsystemPath := filepath.Join(cgroupRoot, subsystem, spec.Name)

		if subsystem == "cpu" {
			if err := setSubsystemValue(subsystemPath, "cpu.shares", spec.Resources.CPU.Shares); err != nil {
				return nil, err
			}
		} else if subsystem == "memory" {
			if err := setSubsystemValue(subsystemPath, "memory.limit_in_bytes", spec.Resources.Memory.Limit); err != nil {
				return nil, err
			}
		} else if subsystem == "blkio" {
			if err := setSubsystemValue(subsystemPath, "blkio.weight", spec.Resources.BlkIO.Weight); err != nil {
				return nil, err
			}
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
	cgroupConfig := &CgroupSpec{
		Name: containerID,
		Resources: &Resources{
			Memory: &Memory{
				Limit: 1024 * 1024 * 1024, // 1 GB
			},
		},
	}
	cgroup, err := NewCgroup(cgroupConfig)
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
