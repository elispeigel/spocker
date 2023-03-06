package container

import (
	"os"
	"syscall"
	"testing"
)

func TestNamespaceEnterAndClose(t *testing.T) {
    spec := &NamespaceSpec{
        Name: "test-namespace",
        Type: NamespaceTypePID,
    }

    ns, err := NewNamespace(spec)
    if err != nil {
        t.Fatalf("failed to create namespace: %v", err)
    }
    defer ns.Close()

    err = ns.Enter()
    if err != nil {
        t.Fatalf("failed to enter namespace: %v", err)
    }
}

func TestMustSetHostname(t *testing.T) {
    err := syscall.Sethostname([]byte("test-hostname"))
    if err != nil {
        t.Fatalf("failed to set hostname: %v", err)
    }

    MustSetHostname("test-hostname2")

    hostname, err := os.Hostname()
    if err != nil {
        t.Fatalf("failed to get hostname: %v", err)
    }
    if hostname != "test-hostname2" {
        t.Fatalf("expected hostname to be %q, but got %q", "test-hostname2", hostname)
    }
}
