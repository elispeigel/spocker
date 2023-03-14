package container

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

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

// GetCgroupParam returns the value of the given cgroup parameter.
func GetCgroupParam(cgroupPath string, param string) (string, error) {
	filePath := filepath.Join(cgroupPath, param)
	valueBytes, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read cgroup parameter %s: %v", param, err)
	}
	return string(bytes.TrimSpace(valueBytes)), nil
}

// SetCgroupParam sets the value of the given cgroup parameter.
func SetCgroupParam(cgroupPath string, param string, value string) error {
	// Open the cgroup parameter file.
	paramFile := filepath.Join(cgroupPath, param)
	file, err := os.OpenFile(paramFile, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open cgroup parameter file: %v", err)
	}
	defer file.Close()

	// Write the value to the file.
	_, err = file.WriteString(value)
	if err != nil {
		return fmt.Errorf("failed to write cgroup parameter value: %v", err)
	}

	return nil
}

// GetInitProcess returns the init process for the current system.
func GetInitProcess() (*os.Process, error) {
	pid := syscall.Getpid()
	for {
		statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
		statFile, err := os.Open(statPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s: %v", statPath, err)
		}
		defer statFile.Close()

		scanner := bufio.NewScanner(statFile)
		scanner.Scan()
		statLine := scanner.Text()
		statFields := strings.Fields(statLine)
		if len(statFields) < 4 {
			return nil, fmt.Errorf("invalid stat file format: %s", statLine)
		}

		// The process with PID 1 is always the init process.
		if statFields[0] == "1" {
			initPid, err := strconv.Atoi(statFields[0])
			if err != nil {
				return nil, fmt.Errorf("failed to parse init PID: %v", err)
			}
			return os.FindProcess(initPid)
		}

		// The parent PID is the fourth field in the stat file.
		ppid, err := strconv.Atoi(statFields[3])
		if err != nil {
			return nil, fmt.Errorf("failed to parse parent PID: %v", err)
		}

		// If the parent PID is 0, then we've reached the root process.
		if ppid == 0 {
			return nil, fmt.Errorf("failed to find init process")
		}

		pid = ppid
	}
}

// ExecContainer runs the container process inside its namespaces.
func ExecContainer(containerID string, command []string) error {
	// Set up namespaces
	cmd := exec.Command(command[0], command[1:]...)
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
	// if err := cgroup.AddProcess(os.Getpid()); err != nil {
	// 	return err
	// }
	defer cgroup.Close()

	// Start the container process
	runErr := cmd.Run()

	if runErr != nil {
		return fmt.Errorf("failed to execute container process: %v", runErr)
	}

	return nil
}
