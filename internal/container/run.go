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

func Run(cmd *exec.Cmd, config *Config) error {
	logger, _ := zap.NewProduction()
	defer func() {
		if err := logger.Sync(); err != nil {
			fmt.Printf("Error syncing logger: %v\n", err)
		}
	}()

	// Set up cgroups, namespaces, or any other container settings here
	subsystems := []cgroup.Subsystem{&cgroup.CPUSubsystem{}, &cgroup.MemorySubsystem{}, &cgroup.BlkIOSubsystem{}}
	fileHandler := &cgroup.DefaultFileHandler{}
	factory := cgroup.NewDefaultFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(&config.CgroupConfig)
	if err != nil {
		logger.Error("Failed to create cgroup", zap.Error(err))
		return fmt.Errorf("failed to create cgroup: %w", err)
	}
	defer cgroup.Close()

	container_namespace, err := namespace.NewNamespace(&config.NamespaceConfig)
	if err != nil {
		logger.Error("Failed to create namespace", zap.Error(err))
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	defer container_namespace.Close()

	// Set up the container's filesystem
	fs, err := filesystem.NewFilesystem(&filesystem.Config{Root: config.FSRoot})
	if err != nil {
		logger.Error("Failed to create filesystem", zap.Error(err))
		return fmt.Errorf("failed to create filesystem: %w", err)
	}

	// Set up the container's network
	networkHandler := network.DefaultNetworkHandler{}
	container_network, err := network.CreateNetwork(&config.NetworkConfig, networkHandler)
	if err != nil {
		logger.Error("Failed to create network", zap.Error(err))
		return fmt.Errorf("failed to create network: %w", err)
	}

	defer func() {
		if err := network.DeleteNetwork(container_network); err != nil {
			logger.Error("Failed to delete network", zap.Error(err))
		}
	}()

	// Configure the container's hostname
	if err := namespace.SetHostname("your-container-hostname"); err != nil {
		logger.Error("Failed to set hostname", zap.Error(err))
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Set up the container's root directory (chroot)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up the container's filesystem before running the command
	cmd.Dir = fs.Root

	// Run the command inside the container
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start command", zap.Error(err))
		return fmt.Errorf("failed to start command: %w", err)
	}

	if _, err := cmd.Process.Wait(); err != nil {
		logger.Error("Failed to wait for command", zap.Error(err))
		return fmt.Errorf("failed to wait for command: %w", err)
	}

	return nil
}

type Config struct {
	CgroupConfig    cgroup.Spec
	NamespaceConfig namespace.NamespaceSpec
	FSRoot          string
	NetworkConfig   network.Config
}