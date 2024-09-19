package container

import (
	"fmt"
	"os/exec"
	"syscall"

	"spocker/internal/container/cgroup"
	"spocker/internal/container/filesystem"
	"spocker/internal/container/namespace"
	"spocker/internal/container/network"
	"spocker/internal/container/seccomp"

	"go.uber.org/zap"
)

type ContainerRunner interface {
	Start() error
	Wait() error
}

func Run(cmd *exec.Cmd, config *Config) error {
	logger := zap.L()
	defer logger.Sync()

	logger.Info("Starting container run process")

	// Set up cgroups
	logger.Debug("Setting up cgroups")
	subsystems := []cgroup.Subsystem{&cgroup.CPUSubsystem{}, &cgroup.MemorySubsystem{}, &cgroup.BlkIOSubsystem{}}
	fileHandler := &cgroup.DefaultFileHandler{}
	factory := cgroup.NewDefaultFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(&config.CgroupConfig)
	if err != nil {
		logger.Error("Failed to create cgroup", zap.Error(err))
		return fmt.Errorf("failed to create cgroup: %w", err)
	}
	defer func() {
		if err := cgroup.Close(); err != nil {
			logger.Error("Failed to close cgroup", zap.Error(err))
		}
	}()

	// Set up namespaces
	logger.Debug("Setting up namespaces")
	container_namespace, err := namespace.NewNamespace(&config.NamespaceConfig)
	if err != nil {
		logger.Error("Failed to create namespace", zap.Error(err))
		return fmt.Errorf("failed to create namespace: %w", err)
	}
	defer func() {
		if err := container_namespace.Close(); err != nil {
			logger.Error("Failed to close namespace", zap.Error(err))
		}
	}()

	// Set up the container's filesystem
	logger.Debug("Setting up filesystem")
	fs, err := filesystem.NewFilesystem(&filesystem.Config{Root: config.FSRoot})
	if err != nil {
		logger.Error("Failed to create filesystem", zap.Error(err))
		return fmt.Errorf("failed to create filesystem: %w", err)
	}

	// Set up the container's network
	logger.Debug("Setting up network")
	networkHandler := network.DefaultNetworkHandler{}
	container_network, err := network.CreateNetwork(&config.NetworkConfig, networkHandler)
	if err != nil {
		logger.Error("Failed to create network", zap.Error(err))
		return fmt.Errorf("failed to create network: %w", err)
	}

	defer func() {
		logger.Debug("Cleaning up network")
		if err := network.DeleteNetwork(container_network); err != nil {
			logger.Error("Failed to delete network", zap.Error(err))
		}
	}()

	// Configure the container's hostname
	logger.Debug("Setting container hostname")
	if err := namespace.SetHostname("your-container-hostname"); err != nil {
		logger.Error("Failed to set hostname", zap.Error(err))
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Set up seccomp filter
	logger.Debug("Applying seccomp filter")
	seccompSpec := &seccomp.SeccompSpec{
		AllowedSyscalls: config.AllowedSyscalls, // You'll need to add this to your Config struct
	}
	if err := seccomp.SetupSeccomp(seccompSpec); err != nil {
		logger.Error("Failed to apply seccomp filter", zap.Error(err))
		return fmt.Errorf("failed to apply seccomp filter: %w", err)
	}

	// Set up the container's root directory (chroot)
	logger.Debug("Setting up container root directory")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up the container's filesystem before running the command
	cmd.Dir = fs.Root

	// Run the command inside the container
	logger.Info("Starting container command")
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start command", zap.Error(err))
		return fmt.Errorf("failed to start command: %w", err)
	}

	logger.Info("Waiting for container command to finish")
	if _, err := cmd.Process.Wait(); err != nil {
		logger.Error("Failed to wait for command", zap.Error(err))
		return fmt.Errorf("failed to wait for command: %w", err)
	}

	logger.Info("Container command finished successfully")
	return nil
}

type Config struct {
	CgroupConfig    cgroup.Spec
	NamespaceConfig namespace.NamespaceSpec
	FSRoot          string
	NetworkConfig   network.Config
	AllowedSyscalls []string
}