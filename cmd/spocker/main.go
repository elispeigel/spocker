package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"spocker/internal/container"
	"spocker/internal/container/cgroup"
	"spocker/internal/container/namespace"
	"spocker/internal/container/network"
	"go.uber.org/zap"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	flag.Usage = usage

	memoryLimitFlag := flag.String("memory-limit", "", "Memory limit for the container in bytes")
	cpuSharesFlag := flag.String("cpu-shares", "", "CPU shares for the container")
	blkioWeightFlag := flag.String("blkio-weight", "", "Block I/O weight for the container")
	cgroupNameFlag := flag.String("cgroup-name", "", "cgroup name for the container")
	namespaceNameFlag := flag.String("namespace-name", "", "namespace name for the container")
	namespaceTypeFlag := flag.Int("namespace-type", 0, "namespace type for the container")
	fsRootFlag := flag.String("fs-root", "", "file system root path for the container")
	networkNameFlag := flag.String("network-name", "", "network name")
	networkIPCIDRFlag := flag.String("network-ip-cidr", "", "network IP CIDR")
	networkGatewayFlag := flag.String("network-gateway", "", "network gateway")

	flag.Parse()

	if len(flag.Args()) == 0 {
		usage()
		os.Exit(1)
	}

	switch flag.Args()[0] {
	case "run":
		memoryLimit, err := strconv.Atoi(*memoryLimitFlag)
		if err != nil {
			logger.Error("spocker: invalid memory limit", zap.String("memory limit", *memoryLimitFlag))
			os.Exit(1)
		}
		cpuShares, err := strconv.Atoi(*cpuSharesFlag)
		if err != nil {
			logger.Error("spocker: invalid CPU shares", zap.String("CPU shares", *cpuSharesFlag))
			os.Exit(1)
		}
		blkioWeight, err := strconv.Atoi(*blkioWeightFlag)
		if err != nil {
			logger.Error("spocker: invalid block I/O weight", zap.String("block I/O weight", *blkioWeightFlag))
			os.Exit(1)
		}

		cgroupSpec := &cgroup.CgroupSpec{
			Name: *cgroupNameFlag,
			Resources: &cgroup.Resources{
				Memory: &cgroup.Memory{
					Limit: memoryLimit,
				},
				CPU: &cgroup.CPU{
					Shares: cpuShares,
				},
				BlkIO: &cgroup.BlkIO{
					Weight: blkioWeight,
				},
			},
		}

		namespaceSpec := &namespace.NamespaceSpec{
			Name: *namespaceNameFlag,
			Type: namespace.NamespaceType(*namespaceTypeFlag),
		}

		_, ipNet, err := net.ParseCIDR(*networkIPCIDRFlag)
		if err != nil {
			logger.Error("Invalid CIDR", zap.String("CIDR", *networkIPCIDRFlag), zap.Error(err))
			return
		}

		networkConfig := &network.NetworkConfig{
			Name:    *networkNameFlag,
			IPNet:   ipNet,
			Gateway: net.ParseIP(*networkGatewayFlag),
		}

		run(cgroupSpec, namespaceSpec, *fsRootFlag, networkConfig)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cgroupSpec *cgroup.CgroupSpec, namespaceSpec *namespace.NamespaceSpec, fsRoot string, networkConfig *network.NetworkConfig) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	if os.Geteuid() != 0 {
		logger.Error("spocker: need root privileges")
		os.Exit(1)
	}

	cmd := exec.Command(flag.Args()[1], flag.Args()[2:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := container.Run(cmd, cgroupSpec, namespaceSpec, fsRoot, networkConfig); err != nil {
		logger.Error("spocker: error running container", zap.Error(err))
		os.Exit(1)
	}
}
