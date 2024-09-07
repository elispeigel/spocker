package process

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"spocker/internal/container/util"
)

// Process represents a container process.
type Process struct {
	cmd *exec.Cmd
}

// NewProcess creates a new container process based on the given ProcessSpec.
func NewProcess(spec *ProcessSpec) (*Process, error) {
	ctx := context.Background()
	cmd, err := util.CreateCommand(ctx, spec.Path, spec.Args...)

	if err := dropPrivileges(spec); err != nil {
		return nil, fmt.Errorf("failed to drop privileges: %w", err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create command: %w", err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	return &Process{cmd: cmd}, nil
}


func dropPrivileges(spec *ProcessSpec) error {
	if spec.User != "" {
		userInfo, err := user.Lookup(spec.User)
		if err != nil {
			return fmt.Errorf("failed to look up user %s: %w", spec.User, err)
		}

		uid, err := strconv.Atoi(userInfo.Uid)
		if err != nil {
			return fmt.Errorf("failed to parse UID: %w", err)
		}

		gid, err := strconv.Atoi(userInfo.Gid)
		if err != nil {
			return fmt.Errorf("failed to parse GID: %w", err)
		}

		if err := syscall.Setgid(gid); err != nil {
			return fmt.Errorf("failed to set GID: %w", err)
		}

		if err := syscall.Setuid(uid); err != nil {
			return fmt.Errorf("failed to set UID: %w", err)
		}
	}

	return nil
}

// Start begins the execution of the container process.
func (p *Process) Start() error {
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start container process: %w", err)
	}
	return nil
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
	if err := p.cmd.Process.Signal(sig); err != nil {
		return fmt.Errorf("failed to send signal to container process: %w", err)
	}
	return nil
}

// ProcessSpec defines the specification for a container process.
type ProcessSpec struct {
	Path string
	Args []string
	User string
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
			return nil, fmt.Errorf("failed to open %s: %w", statPath, err)
		}
		defer func() {
			if err := statFile.Close(); err != nil {
				zap.L().Error("Failed to close stat file", zap.String("path", statPath), zap.Error(err))
			}
		}()

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
				return nil, fmt.Errorf("failed to parse init PID: %w", err)
			}
			return os.FindProcess(initPid)
		}

		// The parent PID is the fourth field in the stat file.
		ppid, err := strconv.Atoi(statFields[3])
		if err != nil {
			return nil, fmt.Errorf("failed to parse parent PID: %w", err)
		}

		// If the parent PID is 0, then we've reached the root process.
		if ppid == 0 {
			return nil, fmt.Errorf("failed to find init process")
		}

		pid = ppid
	}
}