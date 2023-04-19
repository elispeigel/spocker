/**
* Package container provides functionalities for setting up a container environment, including cgroups,
* namespaces, filesystems, networks, and running commands inside the container. The Run function sets up
* the container environment and runs the specified command. It uses cgroups to manage system resources,
* namespaces to isolate the container's processes, filesystems to isolate the container's filesystem, and
* networks to isolate the container's network stack.
*/
package container

import (
	"fmt"
	// "os"
	"os/exec"
	"syscall"
)

// Run sets up the container environment and runs the specified command.
func Run(cmd *exec.Cmd, cgroupSpec *CgroupSpec, namespaceSpec *NamespaceSpec, fsRoot string, networkConfig *NetworkConfig) error {
	// Set up cgroups, namespaces, or any other container settings here
	subsystems := []Subsystem{&CPUSubsystem{}, &MemorySubsystem{}, &BlkIOSubsystem{}}
	factory := NewDefaultCgroupFactory(subsystems)
	cgroup, err := factory.CreateCgroup(cgroupSpec)
	if err != nil {
		return fmt.Errorf("failed to create cgroup: %v", err)
	}
	defer cgroup.Close()

	namespace, err := NewNamespace(namespaceSpec)
	if err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}
	defer namespace.Close()

	// Set up the container's filesystem
	fs, err := NewFilesystem(fsRoot)
	if err != nil {
		return fmt.Errorf("failed to create filesystem: %v", err)
	}

	// Set up the container's network
	network, err := CreateNetwork(networkConfig)
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}
	defer DeleteNetwork(network.Name)

	// Configure the container's hostname
	if err := SetHostname("your-container-hostname"); err != nil {
		return fmt.Errorf("failed to set hostname: %v", err)
	}

	// Set up the container's root directory (chroot)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up the container's filesystem before running the command
	cmd.Dir = fs.Root

	// Run the command inside the container
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	if _, err := cmd.Process.Wait(); err != nil {
		return fmt.Errorf("failed to wait for command: %v", err)
	}

	return nil
}
