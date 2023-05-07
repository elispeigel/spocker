// cgroup package manages Linux control groups (cgroups) and provides functionality to apply resource limitations.
package cgroup

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// NewCPUSubsystem initializes a new CPUSubsystem instance with the provided fileHandler.
func NewCPUSubsystem(fileHandler FileHandler) *CPUSubsystem {
	return &CPUSubsystem{fileHandler: fileHandler}
}

// Name returns the name of the CPUSubsystem, which is "cpu".
func (c *CPUSubsystem) Name() string {
	return "cpu"
}

// ApplySettings applies the provided CPU resources settings to the specified cgroup path.
func (c *CPUSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(c.fileHandler, cgroupPath, "cpu.shares", resources.CPU.Shares)
}

// NewMemorySubsystem initializes a new MemorySubsystem instance with the provided fileHandler.
func NewMemorySubsystem(fileHandler FileHandler) *MemorySubsystem {
	return &MemorySubsystem{fileHandler: fileHandler}
}

// Name returns the name of the MemorySubsystem, which is "memory".
func (m *MemorySubsystem) Name() string {
	return "memory"
}

// ApplySettings applies the provided memory resources settings to the specified cgroup path.
func (m *MemorySubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(m.fileHandler, cgroupPath, "memory.limit_in_bytes", resources.Memory.Limit)
}

// NewBlkIOSubsystem initializes a new BlkIOSubsystem instance with the provided fileHandler.
func NewBlkIOSubsystem(fileHandler FileHandler) *BlkIOSubsystem {
	return &BlkIOSubsystem{fileHandler: fileHandler}
}

// Name returns the name of the BlkIOSubsystem, which is "blkio".
func (b *BlkIOSubsystem) Name() string {
	return "blkio"
}

// ApplySettings applies the provided block I/O resources settings to the specified cgroup path.
func (b *BlkIOSubsystem) ApplySettings(cgroupPath string, resources *Resources) error {
	return setSubsystemValue(b.fileHandler, cgroupPath, "blkio.weight", resources.BlkIO.Weight)
}

// setSubsystemValue sets the value of the specified cgroup subsystem file, handling errors if the file cannot be opened or written to.
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
