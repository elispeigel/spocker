package network

import (
	"context"
	"fmt"

	"github.com/containernetworking/cni/libcni"
	"go.uber.org/zap"
)

func SetupCNINetwork(config *Config) (*libcni.NetworkConfigList, error) {
	cniConfig := &libcni.CNIConfig{
		Path: config.CNIPluginPaths,
	}

	networkConfig, err := libcni.ConfListFromBytes([]byte(config.CNIConfigJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse CNI config JSON: %w", err)
	}

	result, err := cniConfig.AddNetworkList(context.Background(), networkConfig, config.RuntimeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to set up CNI network: %w", err)
	}

	zap.L().Info("CNI network set up successfully", zap.Any("result", result))

	return networkConfig, nil
}

func CleanupCNINetwork(networkConfigList *libcni.NetworkConfigList, pluginPaths []string) error {
    cniConfig := &libcni.CNIConfig{
        Path: pluginPaths, // Use pluginPaths directly as it is now passed as an argument
    }

    if err := cniConfig.DelNetworkList(context.Background(), networkConfigList, nil); err != nil {
        return fmt.Errorf("failed to clean up CNI network: %w", err)
    }

    zap.L().Info("CNI network cleaned up successfully")

    return nil
}

