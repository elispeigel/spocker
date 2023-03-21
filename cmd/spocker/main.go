package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"strconv"
	"github.com/elispeigel/spocker/internal/container"
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
	namespaceTypeFlag := flag.String("namespace-type", "", "namespace type for the container")


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
		namespaceTypeFlag, err := strconv.Atoi(*namespaceTypeFlag)
		if err!= nil {
            fmt.Fprintf(os.Stderr, "spocker: invalid namespace type: %s\n", *namespaceTypeFlag)
            os.Exit(1)
        }
	
		cgroupSpec := &container.CgroupSpec{
			Name: *cgroupNameFlag,
			Resources: &container.Resources{
				Memory: &container.Memory{
					Limit: memoryLimit,
				},
				CPU: &container.CPU{
					Shares: cpuShares,
				},
				BlkIO: &container.BlkIO{
					Weight: blkioWeight,
				},
			},
		}

		namespaceSpec := &container.NamespaceSpec{
			Name: *namespaceNameFlag,
			Type: *namespaceTypeFlag,
		}


		run(cgroupSpec, namespaceSpec,)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cgroupSpec *container.CgroupSpec, namespaceSpec *container.NamespaceSpec) {
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

	if err := container.Run(cmd, cgroupSpec, namespaceSpec); err != nil {
		fmt.Fprintf(os.Stderr, "spocker: %v\n", err)
		os.Exit(1)
	}
}
