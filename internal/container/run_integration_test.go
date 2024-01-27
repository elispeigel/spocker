//go:build integration
// +build integration

package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"spocker/internal/container/cgroup"
)

func TestRunAttachesProcessToCgroup(t *testing.T) {
	// This test requires root/CAP_SYS_ADMIN
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges")
	}

	cgroupSpec := &cgroup.Spec{
		Name: "test-spocker-cgroup",
		Resources: &cgroup.Resources{
			Memory: &cgroup.Memory{
				Limit: 100 * 1024 * 1024, // 100MB
			},
			CPU: &cgroup.CPU{
				Shares: 512,
			},
		},
	}

	// Create a simple command that will run long enough for us to check cgroup
	cmd := exec.Command("/bin/sleep", "5")

	// Start the container in a goroutine so we can check while it's running
	errChan := make(chan error, 1)
	go func() {
		errChan <- Run(cmd, cgroupSpec, nil, "/tmp", nil)
	}()

	// Give the process time to start and be added to cgroup
	// Wait a bit for the process to start
	for i := 0; i < 10; i++ {
		if cmd.Process != nil {
			break
		}
		// Sleep for 100ms between checks
		exec.Command("/bin/sleep", "0.1").Run()
	}

	if cmd.Process == nil {
		t.Fatal("Process did not start")
	}

	// Verify process was added to cgroup while it's still running
	cgroupProcs := filepath.Join("/sys/fs/cgroup", cgroupSpec.Name, "cgroup.procs")
	data, err := os.ReadFile(cgroupProcs)
	if err != nil {
		t.Fatalf("Could not read cgroup.procs: %v", err)
	}

	if len(data) == 0 {
		t.Error("No processes attached to cgroup")
	}

	// Wait for the container to finish
	if err := <-errChan; err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}
