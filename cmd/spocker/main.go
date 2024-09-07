package main

import (
	"flag"
	"fmt"
	"os"

	"spocker/internal/container/cgroup"
	"spocker/internal/container/namespace"
	"spocker/internal/container/network"

	"go.uber.org/zap"
)

type Config struct {
	CgroupConfig    cgroup.Spec
	NamespaceConfig namespace.NamespaceSpec
	NetworkConfig   network.Config
	NetworkIPCIDR   string
	NetworkGateway  string
	FSRoot          string
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Printf("Error setting up logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	zap.ReplaceGlobals(logger)

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

	switch flag.Args()[0] {
	case "run":
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
	config := &Config{}

	flag.IntVar(&config.CgroupConfig.Resources.Memory.Limit, "memory-limit", 0, "Memory limit for the container in bytes")
	flag.IntVar(&config.CgroupConfig.Resources.CPU.Shares, "cpu-shares", 0, "CPU shares for the container")
	flag.IntVar(&config.CgroupConfig.Resources.BlkIO.Weight, "blkio-weight", 0, "Block I/O weight for the container")
	flag.StringVar(&config.CgroupConfig.Name, "cgroup-name", "", "cgroup name for the container")
	flag.StringVar(&config.NamespaceConfig.Name, "namespace-name", "", "namespace name for the container")
	flag.IntVar((*int)(&config.NamespaceConfig.Type), "namespace-type", 0, "namespace type for the container")
	flag.StringVar(&config.FSRoot, "fs-root", "", "file system root path for the container")
	flag.StringVar(&config.NetworkConfig.Name, "network-name", "", "network name")
	flag.StringVar(&config.NetworkIPCIDR, "network-ip-cidr", "", "network IP CIDR")
	flag.StringVar(&config.NetworkGateway, "network-gateway", "", "network gateway")
	
	flag.Parse()

	if config.CgroupConfig.Name == "" {
		return nil, fmt.Errorf("required flag 'cgroup-name' not set")
	}

	if config.NetworkConfig.Name == "" {
		return nil, fmt.Errorf("required flag 'network-name' not set")
	}

	zap.L().Info("Parsed configuration",
		zap.Int("memory-limit", config.CgroupConfig.Resources.Memory.Limit),
		zap.Int("cpu-shares", config.CgroupConfig.Resources.CPU.Shares),
		zap.Int("blkio-weight", config.CgroupConfig.Resources.BlkIO.Weight),
		zap.String("cgroup-name", config.CgroupConfig.Name),
		zap.String("namespace-name", config.NamespaceConfig.Name),
		zap.Int("namespace-type", int(config.NamespaceConfig.Type)),
		zap.String("fs-root", config.FSRoot),
		zap.String("network-name", config.NetworkConfig.Name),
		zap.String("network-ip-cidr", config.NetworkIPCIDR),
		zap.String("network-gateway", config.NetworkGateway),
	)

	return config, nil
}

func runContainer(config *Config, logger *zap.Logger) error {
	logger.Info("Running container", zap.String("cgroup-name", config.CgroupConfig.Name))

	// TODO: Implement container running logic here
	// This is where you would use the parsed configuration to set up and run the container

	logger.Info("Container execution completed")
	return nil
}