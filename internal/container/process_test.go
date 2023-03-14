package container

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
        t.Errorf("NewProcess returned an error: %v", err)
    }
    if proc == nil {
        t.Errorf("NewProcess returned nil")
    }
}

func TestStartWait(t *testing.T) {
    spec := &ProcessSpec{
        Path: "/bin/bash",
        Args: []string{"-c", "echo hello"},
    }
    proc, err := NewProcess(spec)
    if err != nil {
        t.Errorf("NewProcess returned an error: %v", err)
    }
    if proc == nil {
        t.Errorf("NewProcess returned nil")
    }
    if err := proc.Start(); err != nil {
        t.Errorf("Start returned an error: %v", err)
    }
    exitCode, err := proc.Wait()
    if err != nil {
        t.Errorf("Wait returned an error: %v", err)
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
        t.Errorf("NewProcess returned an error: %v", err)
    }
    if proc == nil {
        t.Errorf("NewProcess returned nil")
    }
    if err := proc.Start(); err != nil {
        t.Errorf("Start returned an error: %v", err)
    }
    if err := proc.Kill(os.Interrupt); err != nil {
        t.Errorf("Kill returned an error: %v", err)
    }
    exitCode, err := proc.Wait()
    if err == nil {
        t.Errorf("Wait did not return an error")
    }
    if exitCode != -1 {
        t.Errorf("Process exited with status %d", exitCode)
    }
}
