package namespace

import (
	"context"
	"fmt"
	"os"
	"syscall"
)

// NewNamespace returns a new namespace object.
func NewNamespace(spec *NamespaceSpec) (*Namespace, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	ctx := context.Background()
	cmd, err := createCommand(ctx, "/proc/self/exe", "child")
	if err != nil {
		return nil, fmt.Errorf("failed to create child process: %w", err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start container process: %w", err)
	}

	file := os.NewFile(uintptr(r.Fd()), "namespace")

	ns := &Namespace{
		Name: spec.Name,
		Type: spec.Type,
		File: file,
	}

	defer file.Close()

	return ns, nil
}

// Namespace is an abstraction over a Linux namespace.
type Namespace struct {
	Name string
	Type NamespaceType
	File *os.File
}

// Enter enters the namespace.
func (ns *Namespace) Enter() error {
	if err := syscall.Dup2(int(ns.File.Fd()), syscall.Stdin); err != nil {
		return fmt.Errorf("failed to duplicate file descriptor to stdin: %w", err)
	}

	ctx := context.Background()
	cmd, err := createCommand(ctx, "/bin/sh", "-i")
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
	Name string
	Type NamespaceType
}

// SetHostname sets the hostname of the current namespace and returns an error if it fails.
func SetHostname(hostname string) error {

	ctx := context.Background()
	cmd, err := createCommand(ctx, "sudo", "hostnamectl", "set-hostname", hostname)
	if err != nil {
		return fmt.Errorf("failed to create command: %w", err)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set hostname to %s: %w", hostname, err)
	}
	return nil
}
