package cgroup

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// Subsystem represents a cgroup subsystem.
type Subsystem interface {
	Name() string
	ApplySettings(cgroupPath string, resources *Resources) error
}

// CPUSubsystem is an implementation of the Subsystem interface for the "cpu" subsystem.
type CPUSubsystem struct {
	fileHandler FileHandler
}

func NewCPUSubsystem(fileHandler FileHandler) *CPUSubsystem {
	return &CPUSubsystem{fileHandler: fileHandler}
}

func (c *CPUSubsystem) Name() string {
	return "cpu"
}

func (c *CPUSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(c.fileHandler, cgroupPath, "cpu.shares", resources.CPU.Shares)
}

// MemorySubsystem is an implementation of the Subsystem interface for the "memory" subsystem.
type MemorySubsystem struct {
	fileHandler FileHandler
}

func NewMemorySubsystem(fileHandler FileHandler) *MemorySubsystem {
	return &MemorySubsystem{fileHandler: fileHandler}
}

func (m *MemorySubsystem) Name() string {
	return "memory"
}

func (m *MemorySubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(m.fileHandler, cgroupPath, "memory.limit_in_bytes", resources.Memory.Limit)
}

// BlkIOSubsystem is an implementation of the Subsystem interface for the "blkio" subsystem.
type BlkIOSubsystem struct {
	fileHandler FileHandler
}

func NewBlkIOSubsystem(fileHandler FileHandler) *BlkIOSubsystem {
	return &BlkIOSubsystem{fileHandler: fileHandler}
}

func (b *BlkIOSubsystem) Name() string {
	return "blkio"
}

func (b *BlkIOSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(b.fileHandler, cgroupPath, "blkio.weight", resources.BlkIO.Weight)
}

func setSubsystemValue(fileHandler FileHandler, subsystemPath, filename string, value int) error {
	subsystemFile, err := fileHandler.OpenFile(filepath.Join(subsystemPath, filename), os.O_WRONLY, 0644)
	if err != nil {
		zap.L().Error("failed to open cgroup subsystem file", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to open %s for cgroup: %v", filename, err)
	}
	defer subsystemFile.Close()
	if _, err := fmt.Fprintf(subsystemFile, "%d", value); err != nil {
		zap.L().Error("failed to set cgroup subsystem value", zap.String("filename", filename), zap.Error(err))
		return fmt.Errorf("failed to set %s value for cgroup: %v", filename, err)
	}
	return nil
}