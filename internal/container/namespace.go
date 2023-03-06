package container

import (
	"fmt"
    "os"
    "os/exec"
    "syscall"
)

// NewNamespace returns a new namespace object.
func NewNamespace(spec *NamespaceSpec) (*Namespace, error) {
    // Create a new pipe to communicate between parent and child processes
    r, w, err := os.Pipe()
    if err != nil {
        return nil, fmt.Errorf("failed to create pipe: %v", err)
    }

    // Create the child process
    cmd := exec.Command("/proc/self/exe", "child")
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
        Unshareflags: syscall.CLONE_NEWNS,
    }
    cmd.ExtraFiles = []*os.File{w}
    cmd.Stderr = os.Stderr

    // Start the child process
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start container process: %v", err)
    }

    // Read the namespace file descriptor from the pipe
    file := os.NewFile(uintptr(r.Fd()), "namespace")
    defer file.Close()

    // Create a new namespace object
    ns := &Namespace{
        Name: spec.Name,
        Type: spec.Type,
        File: file,
    }

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
    // Duplicate the namespace file descriptor to the standard input
    if err := syscall.Dup2(int(ns.File.Fd()), syscall.Stdin); err != nil {
        return fmt.Errorf("failed to duplicate file descriptor to stdin: %v", err)
    }
    
    // Run the "sh" command as a new process with the "bash" shell
    if err := exec.Command("/bin/sh", "-i").Run(); err != nil {
        return fmt.Errorf("failed to start shell: %v", err)
    }

    return nil
}


// Close releases the namespace's resources.
func (ns *Namespace) Close() error {
    // Close the namespace file descriptor
    if err := ns.File.Close(); err != nil {
        return fmt.Errorf("failed to close namespace file: %v", err)
    }

    return nil
}


// NamespaceType is an enumeration of the different types of Linux namespaces.
type NamespaceType int

// Enumeration of Linux namespace types.
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

// MustSetHostname sets the hostname of the current namespace.
func MustSetHostname(hostname string) {
    if err := syscall.Sethostname([]byte(hostname)); err != nil {
        panic(fmt.Sprintf("failed to set hostname to %s: %v", hostname, err))
    }
}

