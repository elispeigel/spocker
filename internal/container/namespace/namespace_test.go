package namespace

import (
	"os"
	"syscall"
	"testing"
)

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewNamespace(t *testing.T) {
	spec := &NamespaceSpec{
		Name: "test-namespace",
		Type: NamespaceTypePID,
	}

	ns, err := NewNamespace(spec)
	assertNoError(t, err)
	defer ns.Close()
}

func TestNamespaceEnterAndClose(t *testing.T) {
	spec := &NamespaceSpec{
		Name: "test-namespace",
		Type: NamespaceTypePID,
	}

	ns, err := NewNamespace(spec)
	assertNoError(t, err)
	defer ns.Close()

	err = ns.Enter()
	assertNoError(t, err)
}

func TestSetHostname(t *testing.T) {
	err := syscall.Sethostname([]byte("test-hostname"))
	assertNoError(t, err)

	err = SetHostname("test-hostname2")
	assertNoError(t, err)

	hostname, err := os.Hostname()
	assertNoError(t, err)

	if hostname != "test-hostname2" {
		t.Fatalf("expected hostname to be %q, but got %q", "test-hostname2", hostname)
	}
}
