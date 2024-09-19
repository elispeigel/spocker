// internal/container/cgroup/factory.go
package cgroup

import (
	"fmt"
)

func NewCPUSubsystem() *CPUSubsystem {
	return &CPUSubsystem{}
}

func NewMemorySubsystem() *MemorySubsystem {
	return &MemorySubsystem{}
}

func NewBlkIOSubsystem() *BlkIOSubsystem {
	return &BlkIOSubsystem{}
}

func setSubsystemValue(fileHandler FileHandler, subsystemPath, filename string, value int) error {
	subsystemFile, err := fileHandler.OpenFile(filepath.Join(subsystemPath, filename), os.O_WRONLY, 0644)
	if err != nil {
		zap.L().Error("Failed to open cgroup subsystem file", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to open %s for cgroup: %w", filename, err)
	}
	defer subsystemFile.Close()
	if _, err := fmt.Fprintf(subsystemFile, "%d", value); err != nil {
		zap.L().Error("Failed to set cgroup subsystem value", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to set %s value for cgroup: %w", filename, err)
	}
	return nil
}
