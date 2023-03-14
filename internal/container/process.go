package container

import (
    "os"
    "os/exec"
    "syscall"
)

// Process is a struct representing a container process.
type Process struct {
    cmd *exec.Cmd
}

// NewProcess creates a new container process.
func NewProcess(spec *ProcessSpec) (*Process, error) {
    cmd := exec.Command(spec.Path, spec.Args...)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
        Unshareflags: syscall.CLONE_NEWNS,
    }

    return &Process{cmd: cmd}, nil
}

// Start starts the container process.
func (p *Process) Start() error {
    return p.cmd.Start()
}

// Wait waits for the container process to exit and returns its exit code.
func (p *Process) Wait() (int, error) {
    err := p.cmd.Wait()
    if err != nil {
        exitErr, ok := err.(*exec.ExitError)
        if !ok {
            return 0, err
        }
        status, ok := exitErr.Sys().(syscall.WaitStatus)
        if !ok {
            return 0, err
        }
        return status.ExitStatus(), nil
    }
    return 0, nil
}

// Kill sends a signal to the container process.
func (p *Process) Kill(sig os.Signal) error {
    return p.cmd.Process.Signal(sig)
}

// ProcessSpec represents the specification for a container process.
type ProcessSpec struct {
    Path string
    Args []string
}
