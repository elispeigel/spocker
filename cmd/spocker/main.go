// cmd/spocker/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"spocker/internal/container/cgroup"
	"spocker/internal/container/filesystem"
	"spocker/internal/container/network"
	"spocker/internal/container/namespace"
	"spocker/internal/container/process"
	"spocker/internal/container/security"
	"spocker/internal/container/util"

	"go.uber.org/zap"
)

type Config struct {
	CgroupConfig    *cgroup.Spec
	NamespaceConfig *namespace.NamespaceSpec
	NetworkConfig   *network.NetworkConfig
	FSRoot          string
	ProcessSpec     *process.ProcessSpec
	SecurityConfig  *security.SecuritySpec
}

func main() {
	// Initialize Logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Error setting up logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	zap.ReplaceGlobals(logger)

	// Parse Command-Line Flags
	config, err := parseFlags()
	if err != nil {
		logger.Error("Error parsing flags", zap.Error(err))
		flag.Usage()
		os.Exit(1)
	}

	if len(flag.Args()) == 0 {
		logger.Error("No command provided")
		flag.Usage()
		os.Exit(1)
	}

	// Handle Signals for Graceful Shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Run based on the provided command
	switch flag.Args()[0] {
	case "run":
		go func() {
			<-sigs
			logger.Info("Received interrupt signal, shutting down...")
			os.Exit(0)
		}()
		if err := runContainer(config, logger); err != nil {
			logger.Error("Failed to run container", zap.Error(err))
			os.Exit(1)
		}
	default:
		logger.Error("Unknown command", zap.String("command", flag.Args()[0]))
		flag.Usage()
		os.Exit(1)
	}
}

func parseFlags() (*Config, error) {
	config := &Config{
		CgroupConfig:    &cgroup.Spec{},
		NamespaceConfig: &namespace.NamespaceSpec{},
		NetworkConfig:   &network.NetworkConfig{},
		ProcessSpec:     &process.ProcessSpec{},
		SecurityConfig:  &security.SecuritySpec{},
	}

	flag.IntVar(&config.CgroupConfig.Resources.Memory.Limit, "memory-limit", 0, "Memory limit for the container in bytes")
	flag.IntVar(&config.CgroupConfig.Resources.CPU.Shares, "cpu-shares", 0, "CPU shares for the container")
	flag.IntVar(&config.CgroupConfig.Resources.BlkIO.Weight, "blkio-weight", 0, "Block I/O weight for the container")
	flag.StringVar(&config.CgroupConfig.Name, "cgroup-name", "spocker", "Cgroup name for the container")

	flag.StringVar(&config.NamespaceConfig.Name, "namespace-name", "spocker", "Namespace name for the container")
	flag.IntVar((*int)(&config.NamespaceConfig.Type), "namespace-type", int(namespace.NamespaceTypePID), "Namespace type for the container (0: PID, 1: UTS, 2: IPC, 3: NET, 4: USER, 5: CGROUP)")

	flag.StringVar(&config.FSRoot, "fs-root", "/var/spocker/fs", "Filesystem root path for the container")

	flag.StringVar(&config.NetworkConfig.Name, "network-name", "spocker-net", "Network name for the container")
	flag.StringVar(&config.NetworkConfig.IPCIDR, "network-ip-cidr", "192.168.1.0/24", "Network IP CIDR")
	flag.StringVar(&config.NetworkConfig.Gateway, "network-gateway", "192.168.1.1", "Network gateway IP")
	flag.BoolVar(&config.NetworkConfig.DHCP, "network-dhcp", false, "Enable DHCP for container network")

	flag.StringVar(&config.ProcessSpec.Path, "cmd", "/bin/bash", "Command to run in the container")
	cmdArgs := flag.String("args", "", "Arguments for the command (comma-separated)")

	flag.BoolVar(&config.SecurityConfig.EnableNoNewPrivileges, "enable-no-new-privileges", true, "Enable no new privileges for the container")
	flag.BoolVar(&config.SecurityConfig.ReadOnlyRootFS, "readonly-rootfs", false, "Set container root filesystem as read-only")
	flag.StringVar(&config.SecurityConfig.RootFS, "rootfs-path", "/var/spocker/fs", "Path to the root filesystem for read-only mode")

	flag.Parse()

	// Parse command arguments
	if *cmdArgs != "" {
		config.ProcessSpec.Args = splitArgs(*cmdArgs)
	}

	return config, nil
}

func splitArgs(args string) []string {
	// Simple split by comma, can be enhanced to handle quotes etc.
	return strings.Split(args, ",")
}

func runContainer(config *Config, logger *zap.Logger) error {
	logger.Info("Running container", zap.String("cgroup-name", config.CgroupConfig.Name))

	// 1. Setup Cgroups
	logger.Info("Setting up cgroups")
	err := setupCgroups(config)
	if err != nil {
		logger.Error("Failed to setup cgroups", zap.Error(err))
		return err
	}
	defer cleanupCgroups(config.CgroupConfig.Name, logger)

	// 2. Setup Namespaces
	logger.Info("Setting up namespaces")
	err = setupNamespaces(config)
	if err != nil {
		logger.Error("Failed to setup namespaces", zap.Error(err))
		return err
	}
	defer cleanupNamespaces(logger)

	// 3. Setup Filesystem
	logger.Info("Setting up filesystem")
	fs, err := setupFilesystem(config)
	if err != nil {
		logger.Error("Failed to setup filesystem", zap.Error(err))
		return err
	}
	defer cleanupFilesystem(fs, logger)

	// 4. Setup Networking
	logger.Info("Setting up networking")
	networkHandler, err := setupNetworking(config)
	if err != nil {
		logger.Error("Failed to setup networking", zap.Error(err))
		return err
	}
	defer cleanupNetworking(networkHandler, logger)

	// 5. Apply Security Profiles
	logger.Info("Applying security configurations")
	err = applySecurity(config)
	if err != nil {
		logger.Error("Failed to apply security configurations", zap.Error(err))
		return err
	}

	// 6. Execute Process
	logger.Info("Executing container process")
	err = executeProcess(config.ProcessSpec, logger)
	if err != nil {
		logger.Error("Failed to execute process", zap.Error(err))
		return err
	}

	logger.Info("Container execution completed successfully")
	return nil
}

func setupCgroups(config *Config) error {
	// Initialize Cgroup Spec
	cgroupSpec := config.CgroupConfig
	if cgroupSpec.Resources == nil {
		cgroupSpec.Resources = &cgroup.Resources{}
	}
	if cgroupSpec.Resources.Memory == nil {
		cgroupSpec.Resources.Memory = &cgroup.Memory{}
	}
	if cgroupSpec.Resources.CPU == nil {
		cgroupSpec.Resources.CPU = &cgroup.CPU{}
	}
	if cgroupSpec.Resources.BlkIO == nil {
		cgroupSpec.Resources.BlkIO = &cgroup.BlkIO{}
	}

	// Initialize Cgroup Factory
	subsystems := []cgroup.Subsystem{
		cgroup.NewCPUSubsystem(),
		cgroup.NewMemorySubsystem(),
		cgroup.NewBlkIOSubsystem(),
	}
	factory := cgroup.NewDefaultFactory(subsystems, &cgroup.DefaultFileHandler{})

	// Create Cgroup
	cgroupObj, err := factory.CreateCgroup(cgroupSpec)
	if err != nil {
		return err
	}

	// Add current process to cgroup
	err = cgroupObj.AddProcess(os.Getpid())
	if err != nil {
		return err
	}

	// Set resource limits
	err = cgroupObj.SetResourceLimits(cgroupSpec.Resources)
	if err != nil {
		return err
	}

	return nil
}

func cleanupCgroups(cgroupName string, logger *zap.Logger) {
	logger.Info("Cleaning up cgroups", zap.String("cgroup-name", cgroupName))
	subsystems := []cgroup.Subsystem{
		cgroup.NewCPUSubsystem(),
		cgroup.NewMemorySubsystem(),
		cgroup.NewBlkIOSubsystem(),
	}
	factory := cgroup.NewDefaultFactory(subsystems, &cgroup.DefaultFileHandler{})
	cgroupObj, err := factory.CreateCgroup(&cgroup.Spec{Name: cgroupName})
	if err != nil {
		logger.Error("Failed to recreate cgroup for cleanup", zap.Error(err))
		return
	}
	err = cgroupObj.Remove()
	if err != nil {
		logger.Error("Failed to remove cgroup", zap.String("cgroup-name", cgroupName), zap.Error(err))
	}
}

func setupNamespaces(config *Config) error {
	// Initialize Namespace Spec
	namespaceSpec := config.NamespaceConfig

	// Initialize Namespace Handler
	nsHandler, err := namespace.NewNamespace(namespaceSpec)
	if err != nil {
		return err
	}

	// Enter Namespace (this will actually switch the namespaces)
	err = nsHandler.Enter()
	if err != nil {
		return err
	}

	return nil
}

func cleanupNamespaces(logger *zap.Logger) {
	// Currently, nothing specific needs to be done to cleanup namespaces
	// Namespaces are cleaned up when the process exits
	logger.Info("Namespaces cleanup completed")
}

func setupFilesystem(config *Config) (*filesystem.Filesystem, error) {
	fsRoot := config.FSRoot

	// Initialize Filesystem Config
	fsConfig := &filesystem.Config{
		Type: "rootfs",
		Root: fsRoot,
		// Additional mounts and permissions can be set here if needed
	}

	// Initialize Filesystem
	fs, err := filesystem.NewFilesystem(fsConfig)
	if err != nil {
		return nil, err
	}

	// Setup Filesystem (e.g., mount, chroot)
	err = fs.Setup()
	if err != nil {
		return nil, err
	}

	return fs, nil
}

func cleanupFilesystem(fs *filesystem.Filesystem, logger *zap.Logger) {
	logger.Info("Cleaning up filesystem")
	err := fs.Cleanup()
	if err != nil {
		logger.Error("Failed to cleanup filesystem", zap.Error(err))
	}
}

func setupNetworking(config *Config) (*network.NetworkHandler, error) {
	networkConfig := config.NetworkConfig

	// Initialize Network Handler
	netHandler := network.NewNetworkHandler()

	// Create Network
	containerNet, err := networkConfig.Setup(netHandler)
	if err != nil {
		return nil, err
	}

	return containerNet, nil
}

func cleanupNetworking(netHandler *network.NetworkHandler, logger *zap.Logger) {
	logger.Info("Cleaning up networking")
	err := netHandler.Cleanup()
	if err != nil {
		logger.Error("Failed to cleanup networking", zap.Error(err))
	}
}

func applySecurity(config *Config) error {
	securitySpec := config.SecurityConfig

	// Apply No New Privileges
	if securitySpec.EnableNoNewPrivileges {
		err := syscall.Prctl(syscall.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0)
		if err != nil {
			return fmt.Errorf("failed to set no new privileges: %w", err)
		}
	}

	// Set Read-Only Root Filesystem
	if securitySpec.ReadOnlyRootFS {
		err := filesystem.RemountReadOnly(config.FSRoot)
		if err != nil {
			return fmt.Errorf("failed to remount rootfs as read-only: %w", err)
		}
	}

	// Apply Seccomp Profile
	if len(securitySpec.Seccomp.AllowedSyscalls) > 0 {
		err := security.SetupSeccomp(&securitySpec.Seccomp)
		if err != nil {
			return fmt.Errorf("failed to apply seccomp profile: %w", err)
		}
	}

	return nil
}

func executeProcess(procSpec *process.ProcessSpec, logger *zap.Logger) error {
	// Initialize Process Spec
	processSpec := &process.ProcessSpec{
		Path: procSpec.Path,
		Args: procSpec.Args,
		User: procSpec.User,
	}

	// Create Process
	proc, err := process.NewProcess(processSpec)
	if err != nil {
		return fmt.Errorf("failed to create process: %w", err)
	}

	// Start Process
	err = proc.Start()
	if err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Wait for Process to Finish
	exitCode, err := proc.Wait()
	if err != nil {
		return fmt.Errorf("process execution failed: %w", err)
	}

	logger.Info("Process exited",
		zap.Int("exit-code", exitCode),
	)

	if exitCode != 0 {
		return fmt.Errorf("process exited with non-zero status: %d", exitCode)
	}

	return nil
}
