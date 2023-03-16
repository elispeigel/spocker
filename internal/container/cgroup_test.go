package container

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
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
		// Close the cgroup resources
		if err := cgroup.Close(); err != nil {
			t.Errorf("failed to close cgroup resources: %v", err)
		}

		// Remove the cgroup after the test finishes
		if err := cgroup.Remove(); err != nil {
			t.Errorf("failed to remove cgroup: %v", err)
		}
	}()

	t.Run("CPU shares", func(t *testing.T) {
		// Set a limit on CPU shares and verify that it was set correctly
		if err := cgroup.Set("cpu.shares", "512"); err != nil {
			t.Fatalf("failed to set CPU shares: %v", err)
		}
		cpuShares, err := readInt(filepath.Join("/sys/fs/cgroup/cpu", spec.Name, "cpu.shares"))
		if err != nil {
			t.Fatalf("failed to read CPU shares: %v", err)
		}
		if cpuShares != 512 {
			t.Errorf("unexpected CPU shares value: got %d, want %d", cpuShares, 512)
		}
	})

	t.Run("Memory limit", func(t *testing.T) {
		// Set a limit on memory and verify that it was set correctly
		if err := cgroup.Set("memory.limit_in_bytes", "1024"); err != nil {
			t.Fatalf("failed to set memory limit: %v", err)
		}
		memoryLimit, err := readInt(filepath.Join("/sys/fs/cgroup/memory", spec.Name, "memory.limit_in_bytes"))
		if err != nil {
			t.Fatalf("failed to read memory limit: %v", err)
		}
		if memoryLimit != 1024 {
			t.Errorf("unexpected memory limit value: got %d, want %d", memoryLimit, 1024)
		}
	})
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
