//go:build linux
// +build linux

package namespace

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"spocker/internal/container/util"
	"golang.org/x/sys/unix"
)

// BuildCloneFlags constructs clone flags from namespace spec
func BuildCloneFlags(spec *NamespaceSpec) uintptr {
	var flags uintptr
	if spec == nil {
		return 0
	}

	if spec.UTS {
		flags |= unix.CLONE_NEWUTS
	}
	if spec.PID {
		flags |= unix.CLONE_NEWPID
	}
	if spec.MNT {
		flags |= unix.CLONE_NEWNS
	}
	if spec.NET {
		flags |= unix.CLONE_NEWNET
	}
	if spec.IPC {
		flags |= unix.CLONE_NEWIPC
	}
	if spec.User {
		flags |= unix.CLONE_NEWUSER
	}

	return flags
}

// NewNamespace returns a new namespace object.
// NOTE: This function is deprecated in favor of using Cloneflags directly.
// It's kept for backwards compatibility.
func NewNamespace(spec *NamespaceSpec) (*Namespace, error) {
	if spec == nil {
		return &Namespace{}, nil
	}

	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	ctx := context.Background()
	cmd, err := util.CreateCommand(ctx, "/proc/self/exe", "child")
	if err != nil {
		return nil, fmt.Errorf("failed to create child process: %w", err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   BuildCloneFlags(spec),
		Unshareflags: unix.CLONE_NEWNS,
	}
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start container process: %w", err)
	}

	file := os.NewFile(uintptr(r.Fd()), "namespace")

	ns := &Namespace{
		File: file,
	}

	defer file.Close()

	return ns, nil
}

// Namespace is an abstraction over a Linux namespace.
type Namespace struct {
	File *os.File
}

// Enter enters the namespace.
func (ns *Namespace) Enter() error {
	if err := syscall.Dup2(int(ns.File.Fd()), syscall.Stdin); err != nil {
		return fmt.Errorf("failed to duplicate file descriptor to stdin: %w", err)
	}

	ctx := context.Background()
	cmd, err := util.CreateCommand(ctx, "/bin/sh", "-i")
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	return nil
}

// Close releases the namespace's resources.
func (ns *Namespace) Close() error {
	if err := ns.File.Close(); err != nil {
		return fmt.Errorf("failed to close namespace file: %w", err)
	}

	return nil
}

// NamespaceType is an enumeration of the different types of Linux namespaces.
type NamespaceType int

// These constants define the types of namespaces that can be created.
const (
	NamespaceTypePID NamespaceType = iota
	NamespaceTypeUTS
	NamespaceTypeIPC
	NamespaceTypeNet
	NamespaceTypeUser
	NamespaceTypeCgroup
)

// NamespaceSpec represents the specification for a Linux namespace.
type NamespaceSpec struct {
	UTS  bool // UTS namespace (hostname isolation)
	PID  bool // PID namespace (process isolation)
	MNT  bool // Mount namespace (filesystem isolation)
	NET  bool // Network namespace (network isolation)
	IPC  bool // IPC namespace (inter-process communication isolation)
	User bool // User namespace (user/group ID isolation)
}

// SetHostname sets the hostname of the current namespace and returns an error if it fails.
func SetHostname(hostname string) error {

	ctx := context.Background()
	cmd, err := util.CreateCommand(ctx, "sudo", "hostnamectl", "set-hostname", hostname)
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set hostname to %s: %w", hostname, err)
	}
	return nil
}
