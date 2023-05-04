package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Process is a struct representing a container process.// Process represents a container process.
type Process struct {
	cmd *exec.Cmd
}

// NewProcess creates a new container process based on the given ProcessSpec.
func NewProcess(spec *ProcessSpec) (*Process, error) {
	ctx := context.Background()
	cmd, err := createCommand(ctx, spec.Path, spec.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	return &Process{cmd: cmd}, nil
}

// Start begins the execution of the container process.
func (p *Process) Start() error {
	return p.cmd.Start()
}

// Wait waits for the container process to exit and returns its exit code.
func (p *Process) Wait() (int, error) {
	err := p.cmd.Wait()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return 0, fmt.Errorf("failed to get exit status: %w", err)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok {
			return 0, fmt.Errorf("failed to get wait status: %w", err)
		}
		return status.ExitStatus(), nil
	}
	return 0, nil
}

// Kill sends a signal to the container process.
func (p *Process) Kill(sig os.Signal) error {
	return p.cmd.Process.Signal(sig)
}

// ProcessSpec defines the specification for a container process.
type ProcessSpec struct {
	Path string
	Args []string
}

// GetInitProcess returns the init process for the current system.
func GetInitProcess() (*os.Process, error) {
	pid := syscall.Getpid()
	for {
		statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
		_, err := strconv.Atoi(strconv.Itoa(pid))
		if err != nil {
			return nil, fmt.Errorf("invalid PID: %v", pid)
		}
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
