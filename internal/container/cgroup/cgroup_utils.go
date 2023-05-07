package cgroup

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

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

// MustLimitMemory limits the memory usage of the current process.
// This function takes a maximum memory value (in bytes) as an argument and limits the memory usage of the current process accordingly.
func MustLimitMemory(maxMemory int64) {
	const memoryLimitControl = "memory.limit_in_bytes"
	cgroupSpec := NewSpecBuilder().
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
	factory := NewDefaultFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(cgroupSpec)

	if err != nil {
		log.Fatalf("failed to create cgroup: %v", err)
	}
	defer cgroup.Close()
	if err := cgroup.Set(memoryLimitControl, fmt.Sprintf("%d", maxMemory)); err != nil {
		log.Fatalf("failed to set %s for cgroup %s: %v", memoryLimitControl, cgroupSpec.Name, err)
	}
}
