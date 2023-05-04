// Spocker is a lightweight container runtime tool that provides basic
// containerization features. It allows you to run processes within a sandboxed
// environment, isolating them from the host system. Spocker supports
// limiting resources such as memory, CPU shares, and block I/O weight, as well
// as providing namespace isolation and basic networking features.

// The tool is controlled through a command-line interface and accepts various
// flags to customize the container environment. These include flags for setting
// memory limits, CPU shares, block I/O weight, cgroup and namespace names,
// namespace types, filesystem root, and network configuration.

// Spocker requires root privileges to execute and leverages Linux kernel
// features such as cgroups, namespaces, and network namespaces to provide
// containerization.
package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/elispeigel/spocker/internal/container"
	"github.com/elispeigel/spocker/internal/container/cgroup"
	"github.com/elispeigel/spocker/internal/container/namespace"
	"github.com/elispeigel/spocker/internal/container/network"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s COMMAND\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
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
			fmt.Fprintf(os.Stderr, "spocker: invalid memory limit: %s\n", *memoryLimitFlag)
			os.Exit(1)
		}
		cpuShares, err := strconv.Atoi(*cpuSharesFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spocker: invalid CPU shares: %s\n", *cpuSharesFlag)
			os.Exit(1)
		}
		blkioWeight, err := strconv.Atoi(*blkioWeightFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "spocker: invalid block I/O weight: %s\n", *blkioWeightFlag)
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
			fmt.Println("Invalid CIDR:", err)
			return
		}

		networkConfig := &network.NetworkConfig{
			Name:  *networkNameFlag,
			IPNet: ipNet,
			Gateway: net.ParseIP(*networkGatewayFlag),
		}

		run(cgroupSpec, namespaceSpec, *fsRootFlag, networkConfig)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cgroupSpec *cgroup.CgroupSpec, namespaceSpec *namespace.NamespaceSpec, fsRoot string, networkConfig *network.NetworkConfig) {
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "spocker: need root privileges\n")
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
		fmt.Fprintf(os.Stderr, "spocker: %v\n", err)
		os.Exit(1)
	}
}
