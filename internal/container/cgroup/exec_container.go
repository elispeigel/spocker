package cgroup

import (
	"context"
	"fmt"
	"os"
	"spocker/internal/container/util"
	"syscall"
)

// ExecContainer runs the container process inside its namespaces.
func ExecContainer(containerID string, command []string) error {
	// Set up namespaces
	ctx := context.Background()
	cmd, err := util.CreateCommand(ctx, command[0], command[1:]...)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	// Set up cgroup
	cgroupConfig := NewSpecBuilder().
		WithName(containerID).
		WithResources(&Resources{
			Memory: &Memory{
				Limit: 1024 * 1024 * 1024, // 1 GB
			},
		}).
		Build()
	fileHandler := &DefaultFileHandler{}
	subsystems := []Subsystem{
		NewCPUSubsystem(fileHandler),
		NewMemorySubsystem(fileHandler),
		NewBlkIOSubsystem(fileHandler),
	}
	factory := NewDefaultCgroupFactory(subsystems, fileHandler)
	cgroup, err := factory.CreateCgroup(cgroupConfig)

	if err != nil {
		return err
	}
	defer cgroup.Close()

	if err := cgroup.AddProcess(os.Getpid(), fileHandler); err != nil {
		return err
	}
	defer cgroup.Close()

	// Start the container process
	runErr := cmd.Run()

	if runErr != nil {
		return fmt.Errorf("failed to execute container process: %v", runErr)
	}

	return nil
}
