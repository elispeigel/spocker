package container

import (
	"fmt"
	"os/exec"
	"syscall"

	"spocker/internal/container/cgroup"
	"spocker/internal/container/filesystem"
	"spocker/internal/container/namespace"
	"spocker/internal/container/network"

	"go.uber.org/zap"
)

type ContainerRunner interface {
	Start() error
	Wait() error
}

// Run sets up the container environment and runs the specified command.
func Run(cmd *exec.Cmd, cgroupSpec *cgroup.Spec, namespaceSpec *namespace.NamespaceSpec, fsRoot string, networkConfig *network.Config) error {
	logger, _ := zap.NewProduction()
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			fmt.Printf("Error syncing logger: %v\n", syncErr)
		}
	}()
	// Set up cgroups, namespaces, or any other container settings here
	subsystems := []cgroup.Subsystem{&cgroup.CPUSubsystem{}, &cgroup.MemorySubsystem{}, &cgroup.BlkIOSubsystem{}}
	fileHandler := &cgroup.DefaultFileHandler{}
	factory := cgroup.NewDefaultCgroupFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(cgroupSpec)
	if err != nil {
		return fmt.Errorf("failed to create cgroup: %v", err)
	}
	defer cgroup.Close()

	container_namespace, err := namespace.NewNamespace(namespaceSpec)
	if err != nil {
		return fmt.Errorf("failed to create namespace: %v", err)
	}
	defer container_namespace.Close()

	// Set up the container's filesystem
	fs, err := filesystem.NewFilesystem(fsRoot)
	if err != nil {
		return fmt.Errorf("failed to create filesystem: %v", err)
	}

	// Set up the container's network
	networkHandler := network.DefaultNetworkHandler{}
	container_network, err := network.CreateNetwork(networkConfig, networkHandler)
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	defer func() {
		err := network.DeleteNetwork(container_network.Name)
		if err != nil {
			logger.Error("Failed to delete network", zap.Error(err))
		}
	}()

	// Configure the container's hostname
	if err := namespace.SetHostname("your-container-hostname"); err != nil {
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
