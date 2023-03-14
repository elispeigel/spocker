package container

import (
	"os"
	"path/filepath"
	"testing"
	"fmt"
)

func TestCgroup(t *testing.T) {
	spec := &CgroupSpec{
		Name: "testcgroup",
	}

	// Create a new cgroup
	cgroup, err := NewCgroup(spec)
	if err != nil {
		t.Fatalf("failed to create cgroup: %v", err)
	}
	defer func() {
		// Remove the cgroup after the test finishes
		if err := os.RemoveAll(filepath.Join("/sys/fs/cgroup", spec.Name)); err != nil {
			t.Fatalf("failed to remove cgroup: %v", err)
		}
	}()

	// Set a limit on CPU shares and verify that it was set correctly
	if err := cgroup.Set("cpu.shares", "512"); err != nil {
		t.Fatalf("failed to set CPU shares: %v", err)
	}
	cpuShares, err := readInt(filepath.Join("/sys/fs/cgroup/cpu", spec.Name, "cpu.shares"))
	if err != nil {
		t.Fatalf("failed to read CPU shares: %v", err)
	}
	if cpuShares != 512 {
		t.Fatalf("unexpected CPU shares value: got %d, want %d", cpuShares, 512)
	}

	// Set a limit on memory and verify that it was set correctly
	if err := cgroup.Set("memory.limit_in_bytes", "1024"); err != nil {
		t.Fatalf("failed to set memory limit: %v", err)
	}
	memoryLimit, err := readInt(filepath.Join("/sys/fs/cgroup/memory", spec.Name, "memory.limit_in_bytes"))
	if err != nil {
		t.Fatalf("failed to read memory limit: %v", err)
	}
	if memoryLimit != 1024 {
		t.Fatalf("unexpected memory limit value: got %d, want %d", memoryLimit, 1024)
	}
}

func readInt(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var n int64
	if _, err := fmt.Fscanf(f, "%d", &n); err != nil {
		return 0, err
	}

	return n, nil
}
