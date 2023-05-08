package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"

	"spocker/internal/container"
	"spocker/internal/container/cgroup"
	"spocker/internal/container/namespace"
	"spocker/internal/container/network"

	"go.uber.org/zap"
)

type Config struct {
	MemoryLimit    int
	CPUShares      int
	BlkioWeight    int
	CgroupName     string
	NamespaceName  string
	NamespaceType  namespace.NamespaceType
	FSRoot         string
	NetworkName    string
	NetworkIPCIDR  string
	NetworkGateway string
}

// usage prints the command usage information.
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	config, err := parseFlags()
	if err != nil {
		logger.Error("Error parsing flags", zap.Error(err))
		usage()
		os.Exit(1)
	}

	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	switch flag.Args()[0] {
	case "run":
		runContainer(config, logger)
	default:
		usage()
		os.Exit(1)
	}
}

// parseFlags parses command line flags and returns a Config struct.
func parseFlags() (*Config, error) {
	flag.Usage = usage

	memoryLimitFlag := flag.Int("memory-limit", 0, "Memory limit for the container in bytes")
	cpuSharesFlag := flag.Int("cpu-shares", 0, "CPU shares for the container")
	blkioWeightFlag := flag.Int("blkio-weight", 0, "Block I/O weight for the container")
	cgroupNameFlag := flag.String("cgroup-name", "", "cgroup name for the container")
	namespaceNameFlag := flag.String("namespace-name", "", "namespace name for the container")
	namespaceTypeFlag := flag.Int("namespace-type", 0, "namespace type for the container")
	fsRootFlag := flag.String("fs-root", "", "file system root path for the container")
	networkNameFlag := flag.String("network-name", "", "network name")
	networkIPCIDRFlag := flag.String("network-ip-cidr", "", "network IP CIDR")
	networkGatewayFlag := flag.String("network-gateway", "", "network gateway")

	flag.Parse()

	return &Config{
		MemoryLimit:    *memoryLimitFlag,
		CPUShares:      *cpuSharesFlag,
		BlkioWeight:    *blkioWeightFlag,
		CgroupName:     *cgroupNameFlag,
		NamespaceName:  *namespaceNameFlag,
		NamespaceType:  namespace.NamespaceType(*namespaceTypeFlag),
		FSRoot:         *fsRootFlag,
		NetworkName:    *networkNameFlag,
		NetworkIPCIDR:  *networkIPCIDRFlag,
		NetworkGateway: *networkGatewayFlag,
	}, nil
}

// runContainer runs a container using the provided configuration and logger.
func runContainer(config *Config, logger *zap.Logger) {
	cgroupSpec := &cgroup.Spec{
		Name: config.CgroupName,
		Resources: &cgroup.Resources{
			Memory: &cgroup.Memory{
				Limit: config.MemoryLimit,
			},
			CPU: &cgroup.CPU{
				Shares: config.CPUShares,
			},
			BlkIO: &cgroup.BlkIO{
				Weight: config.BlkioWeight,
			},
		},
	}

	namespaceSpec := &namespace.NamespaceSpec{
		Name: config.NamespaceName,
		Type: config.NamespaceType,
	}

	_, ipNet, err := net.ParseCIDR(config.NetworkIPCIDR)
	if err != nil {
		logger.Error("Invalid CIDR", zap.String("CIDR", config.NetworkIPCIDR), zap.Error(err))
		return
	}

	networkConfig := &network.Config{
		Name:    config.NetworkName,
		IPNet:   ipNet,
		Gateway: net.ParseIP(config.NetworkGateway),
	}

	cmd := exec.Command(flag.Args()[1], flag.Args()[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = container.Run(
		cmd,
		cgroupSpec,
		namespaceSpec,
		config.FSRoot,
		networkConfig,
	)
	if err != nil {
		logger.Error("Failed to run container", zap.Error(err))
		return
	}
}
