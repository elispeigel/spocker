package process

import (
	"os"
	"testing"
)

func TestNewProcess(t *testing.T) {
	spec := &ProcessSpec{
		Path: "/bin/bash",
		Args: []string{"-c", "echo hello"},
	}
	proc, err := NewProcess(spec)
	if err != nil {
		t.Fatalf("NewProcess returned an error: %v", err)
	}
	if proc == nil {
		t.Fatal("NewProcess returned nil")
	}
}

func TestStartWait(t *testing.T) {
	spec := &ProcessSpec{
		Path: "/bin/bash",
		Args: []string{"-c", "echo hello"},
	}
	proc, err := NewProcess(spec)
	if err != nil {
		t.Fatalf("NewProcess returned an error: %v", err)
	}
	if err := proc.Start(); err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	exitCode, err := proc.Wait()
	if err != nil {
		t.Fatalf("Wait returned an error: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Process exited with status %d", exitCode)
	}
}

func TestKill(t *testing.T) {
	spec := &ProcessSpec{
		Path: "/bin/sleep",
		Args: []string{"5"},
	}
	proc, err := NewProcess(spec)
	if err != nil {
		t.Fatalf("NewProcess returned an error: %v", err)
	}
	if err := proc.Start(); err != nil {
		t.Fatalf("Start returned an error: %v", err)
	}
	if err := proc.Kill(os.Interrupt); err != nil {
		t.Fatalf("Kill returned an error: %v", err)
	}
	exitCode, err := proc.Wait()
	if err == nil {
		t.Fatal("Wait did not return an error")
	}
	if exitCode != -1 {
		t.Errorf("Process exited with status %d", exitCode)
	}
}
