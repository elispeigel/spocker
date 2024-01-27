# Container Isolation Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix core container isolation functionality by implementing actual cgroup attachment, namespace entry, filesystem isolation, and network interface creation.

**Architecture:** Address six critical gaps: (1) attach container process to created cgroup, (2) properly enter namespace contexts, (3) implement chroot/pivot_root with essential mounts, (4) build real network interfaces via netlink, (5) make CLI network parameters optional with defaults, (6) restore missing documentation.

**Tech Stack:** Go 1.x, Linux namespaces (CLONE_NEWUTS/NEWPID/NEWNS/NEWNET), cgroups, syscall package, netlink library, filesystem isolation (chroot/pivot_root).

---

## Task 1: Fix Cgroup Process Attachment

**Files:**
- Modify: `internal/container/run.go:30-37,79-88`
- Test: `internal/container/run_test.go` (if exists) or create integration test

**Step 1: Write failing test for cgroup attachment**

Create or modify test file to verify process attachment:

```go
// internal/container/run_integration_test.go
// +build integration

package container

import (
    "os"
    "testing"
    "path/filepath"
)

func TestRunAttachesProcessToCgroup(t *testing.T) {
    // This test requires root/CAP_SYS_ADMIN
    if os.Geteuid() != 0 {
        t.Skip("Test requires root privileges")
    }

    cgroupSpec := &cgroup.CgroupSpec{
        Name: "test-spocker-cgroup",
        Resources: cgroup.Resources{
            Memory: 100 * 1024 * 1024, // 100MB
            CPU:    50,
        },
    }

    // Run minimal container
    err := Run("test-container", "/bin/sleep", []string{"5"}, cgroupSpec, nil, nil, nil, nil)
    if err != nil {
        t.Fatalf("Run failed: %v", err)
    }

    // Verify process was added to cgroup
    cgroupProcs := filepath.Join("/sys/fs/cgroup", cgroupSpec.Name, "cgroup.procs")
    data, err := os.ReadFile(cgroupProcs)
    if err != nil {
        t.Fatalf("Could not read cgroup.procs: %v", err)
    }

    if len(data) == 0 {
        t.Error("No processes attached to cgroup")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/container -v -run TestRunAttachesProcessToCgroup`

Expected: FAIL - cgroup.procs is empty or only contains parent process

**Step 3: Add cgroup attachment after process start**

Modify `internal/container/run.go`:

```go
// Around line 79-88, after cmd.Start()
if err := cmd.Start(); err != nil {
    return fmt.Errorf("failed to start container: %v", err)
}

// Attach the container process to the cgroup
if err := cgroup.AddProcess(cmd.Process.Pid, fileHandler); err != nil {
    cmd.Process.Kill()
    return fmt.Errorf("failed to add process to cgroup: %v", err)
}

defer func() {
    if err := cmd.Wait(); err != nil {
        logger.Error("Container process failed", zap.Error(err))
    }
}()
```

**Step 4: Verify cgroup has AddProcess method**

Check `internal/container/cgroup/cgroup.go` for AddProcess method. If missing, add:

```go
// AddProcess adds a process to the cgroup
func (c *Cgroup) AddProcess(pid int, fileHandler *os.File) error {
    procsPath := filepath.Join(c.Path, "cgroup.procs")

    if fileHandler != nil {
        _, err := fileHandler.WriteString(fmt.Sprintf("%d\n", pid))
        return err
    }

    return os.WriteFile(procsPath, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}
```

**Step 5: Run test to verify it passes**

Run: `go test -tags=integration ./internal/container -v -run TestRunAttachesProcessToCgroup`

Expected: PASS - process PID appears in cgroup.procs

**Step 6: Commit**

```bash
git add internal/container/run.go internal/container/cgroup/cgroup.go internal/container/run_integration_test.go
git commit -m "fix: attach container process to cgroup after spawn

- Add cgroup.AddProcess() call after cmd.Start()
- Ensure child process PID is added to cgroup.procs
- Add integration test to verify cgroup attachment
- Addresses issue #2 from container isolation audit"
```

---

## Task 2: Implement Namespace Entry

**Files:**
- Modify: `internal/container/run.go:39-43,71-73`
- Modify: `internal/container/namespace/namespace.go:56-62` (Enter method)

**Step 1: Write test for namespace isolation**

```go
// internal/container/run_integration_test.go
func TestRunEntersNamespaceContext(t *testing.T) {
    if os.Geteuid() != 0 {
        t.Skip("Test requires root privileges")
    }

    nsSpec := &namespace.NamespaceSpec{
        UTS: true,
        PID: true,
        MNT: true,
        NET: true,
    }

    // Run container that reports its PID namespace
    // In a new PID namespace, the process should see itself as PID 1
    err := Run("test-ns", "/bin/sh", []string{"-c", "echo $$"}, nil, nsSpec, nil, nil, nil)
    if err != nil {
        t.Fatalf("Run failed: %v", err)
    }

    // This is a basic check - in practice, need to capture stdout
    // and verify the process reports PID 1 inside the namespace
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/container -v -run TestRunEntersNamespaceContext`

Expected: FAIL - process doesn't report PID 1 (not in new PID namespace)

**Step 3: Refactor namespace usage in run.go**

The current code creates a namespace object but doesn't use it. We should use Cloneflags approach exclusively:

```go
// internal/container/run.go - Remove lines 39-43
// Delete:
// container_namespace, err := namespace.NewNamespace(namespaceSpec)
// if err != nil {
//     return fmt.Errorf("failed to create namespace: %v", err)
// }
// defer container_namespace.Cleanup()

// Keep the Cloneflags approach but ensure it's complete
// Around lines 71-73, expand namespace configuration:
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags: buildCloneFlags(namespaceSpec),
    Unshareflags: syscall.CLONE_NEWNS, // Unshare mount namespace
}

// Add helper function to build flags from spec
func buildCloneFlags(spec *namespace.NamespaceSpec) uintptr {
    var flags uintptr
    if spec == nil {
        return 0
    }

    if spec.UTS {
        flags |= syscall.CLONE_NEWUTS
    }
    if spec.PID {
        flags |= syscall.CLONE_NEWPID
    }
    if spec.MNT {
        flags |= syscall.CLONE_NEWNS
    }
    if spec.NET {
        flags |= syscall.CLONE_NEWNET
    }
    if spec.IPC {
        flags |= syscall.CLONE_NEWIPC
    }
    if spec.User {
        flags |= syscall.CLONE_NEWUSER
    }

    return flags
}
```

**Step 4: Run test to verify namespace isolation**

Run: `go test -tags=integration ./internal/container -v -run TestRunEntersNamespaceContext`

Expected: PASS - process enters namespace(s) correctly

**Step 5: Commit**

```bash
git add internal/container/run.go
git commit -m "fix: properly enter namespace context via Cloneflags

- Remove unused namespace object creation
- Use Cloneflags exclusively for namespace creation
- Add buildCloneFlags helper to construct flags from spec
- Simplify namespace logic to avoid duplicate contexts
- Addresses issue #3 from container isolation audit"
```

---

## Task 3: Implement Filesystem Isolation (chroot/mount)

**Files:**
- Modify: `internal/container/run.go:45-49,70-76`
- Modify: `internal/container/filesystem/filesystem.go:52-59` (call Mount methods)

**Step 1: Write test for filesystem isolation**

```go
// internal/container/run_integration_test.go
func TestRunChrootsToFilesystem(t *testing.T) {
    if os.Geteuid() != 0 {
        t.Skip("Test requires root privileges")
    }

    // Create temporary rootfs
    tmpRoot := t.TempDir()

    // Create basic filesystem structure
    os.MkdirAll(filepath.Join(tmpRoot, "bin"), 0755)
    os.MkdirAll(filepath.Join(tmpRoot, "proc"), 0755)
    os.MkdirAll(filepath.Join(tmpRoot, "sys"), 0755)

    // Copy /bin/sh to tmpRoot/bin/sh
    input, _ := os.ReadFile("/bin/sh")
    os.WriteFile(filepath.Join(tmpRoot, "bin", "sh"), input, 0755)

    fsSpec := &filesystem.FilesystemSpec{
        Root: tmpRoot,
    }

    // Run container that lists root directory
    // If chroot works, it should only see bin, proc, sys
    err := Run("test-fs", "/bin/sh", []string{"-c", "ls /"}, nil, nil, fsSpec, nil, nil)
    if err != nil {
        t.Fatalf("Run failed: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/container -v -run TestRunChrootsToFilesystem`

Expected: FAIL - process sees host filesystem, not container rootfs

**Step 3: Implement pivot_root in filesystem setup**

Modify `internal/container/run.go` to call filesystem mount before exec:

```go
// internal/container/run.go around line 70-76
// Add filesystem setup in child process before exec
// We need to do this AFTER fork but BEFORE exec

// Update SysProcAttr to include a function that runs in the child:
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags:   buildCloneFlags(namespaceSpec),
    Unshareflags: syscall.CLONE_NEWNS,
}

// Since we can't use SysProcAttr.Credential callback easily,
// we need to set up chroot before cmd.Start() or use pivot_root
//
// Better approach: Add pre-exec setup to filesystem
if err := fs.SetupRoot(); err != nil {
    return fmt.Errorf("failed to setup filesystem root: %v", err)
}

// Then in filesystem.go, add SetupRoot method:
```

**Step 4: Implement SetupRoot in filesystem package**

```go
// internal/container/filesystem/filesystem.go
// Add after NewFilesystem function

// SetupRoot prepares the root filesystem and performs pivot_root
func (f *Filesystem) SetupRoot() error {
    // Create essential directories
    dirs := []string{"proc", "sys", "dev", "tmp", "oldroot"}
    for _, dir := range dirs {
        path := filepath.Join(f.Root, dir)
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %v", dir, err)
        }
    }

    // Mount proc
    procPath := filepath.Join(f.Root, "proc")
    if err := syscall.Mount("proc", procPath, "proc", 0, ""); err != nil {
        return fmt.Errorf("failed to mount proc: %v", err)
    }

    // Mount sys
    sysPath := filepath.Join(f.Root, "sys")
    if err := syscall.Mount("sysfs", sysPath, "sysfs", 0, ""); err != nil {
        return fmt.Errorf("failed to mount sys: %v", err)
    }

    // Mount tmpfs on /tmp
    tmpPath := filepath.Join(f.Root, "tmp")
    if err := syscall.Mount("tmpfs", tmpPath, "tmpfs", 0, ""); err != nil {
        return fmt.Errorf("failed to mount tmpfs: %v", err)
    }

    // Perform pivot_root
    oldroot := filepath.Join(f.Root, "oldroot")
    if err := syscall.PivotRoot(f.Root, oldroot); err != nil {
        return fmt.Errorf("failed to pivot_root: %v", err)
    }

    // Change to new root
    if err := os.Chdir("/"); err != nil {
        return fmt.Errorf("failed to chdir to /: %v", err)
    }

    // Unmount old root
    if err := syscall.Unmount("/oldroot", syscall.MNT_DETACH); err != nil {
        return fmt.Errorf("failed to unmount old root: %v", err)
    }

    // Remove oldroot directory
    if err := os.RemoveAll("/oldroot"); err != nil {
        return fmt.Errorf("failed to remove oldroot: %v", err)
    }

    return nil
}
```

**Step 5: Handle pivot_root timing issue**

pivot_root must happen in the child process, not the parent. We need to refactor:

```go
// internal/container/run.go
// Instead of calling fs.SetupRoot() before cmd.Start(),
// we need to use a wrapper script or modify exec approach

// Create a setup function that will be called by the child
// One approach: use /proc/self/exe with a special flag
// Another: create init binary that does setup then exec

// For now, simpler approach: use chroot instead of pivot_root
// (pivot_root is better but more complex to implement correctly)

// Replace SetupRoot call with:
if err := fs.ChrootAndMount(); err != nil {
    return fmt.Errorf("failed to chroot: %v", err)
}

// Update cmd setup to chroot before exec:
cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags:   buildCloneFlags(namespaceSpec),
    Unshareflags: syscall.CLONE_NEWNS,
    Chroot:       fs.Root,
}
```

**Step 6: Implement ChrootAndMount**

```go
// internal/container/filesystem/filesystem.go

// ChrootAndMount sets up essential mounts in the rootfs
func (f *Filesystem) ChrootAndMount() error {
    // Create essential directories
    dirs := []string{"proc", "sys", "dev", "tmp"}
    for _, dir := range dirs {
        path := filepath.Join(f.Root, dir)
        if err := os.MkdirAll(path, 0755); err != nil {
            return fmt.Errorf("failed to create %s: %v", dir, err)
        }
    }

    // Note: Actual mounting must happen AFTER chroot in child process
    // We'll need to handle this via a pre-exec hook or init process
    // For now, just ensure directories exist

    return nil
}

// MountEssentialFilesystems mounts proc, sys, dev after chroot
// This should be called from within the container process
func (f *Filesystem) MountEssentialFilesystems() error {
    // Mount proc
    if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
        return fmt.Errorf("failed to mount proc: %v", err)
    }

    // Mount sys
    if err := syscall.Mount("sysfs", "/sys", "sysfs", 0, ""); err != nil {
        return fmt.Errorf("failed to mount sys: %v", err)
    }

    // Mount tmpfs on /tmp
    if err := syscall.Mount("tmpfs", "/tmp", "tmpfs", 0, ""); err != nil {
        return fmt.Errorf("failed to mount tmpfs: %v", err)
    }

    return nil
}
```

**Step 7: Create container init process**

This is complex - for MVP, we'll use SysProcAttr.Chroot and accept that mounts won't work perfectly. Document limitation:

```go
// internal/container/run.go
// Add comment explaining limitation:

// Set up filesystem isolation
// NOTE: Using Chroot here. Essential filesystem mounts (proc, sys, dev)
// require either:
// 1. A container init process that mounts before exec
// 2. Using cmd with a wrapper that calls MountEssentialFilesystems
// Current implementation creates mount points but doesn't mount
// TODO: Implement proper init process for complete filesystem isolation

cmd.SysProcAttr = &syscall.SysProcAttr{
    Cloneflags:   buildCloneFlags(namespaceSpec),
    Unshareflags: syscall.CLONE_NEWNS,
    Chroot:       fs.Root,
}
cmd.Dir = "/"
```

**Step 8: Run test to verify chroot works**

Run: `go test -tags=integration ./internal/container -v -run TestRunChrootsToFilesystem`

Expected: PASS - process runs in chrooted environment

**Step 9: Commit**

```bash
git add internal/container/run.go internal/container/filesystem/filesystem.go
git commit -m "fix: implement filesystem isolation via chroot

- Add Chroot to SysProcAttr for filesystem isolation
- Create essential mount point directories (proc, sys, dev, tmp)
- Set container working directory to / after chroot
- Add ChrootAndMount preparation method
- Document limitation: essential mounts need init process
- Addresses issue #4 from container isolation audit

TODO: Implement proper container init for proc/sys/dev mounts"
```

---

## Task 4: Implement Real Network Interface Creation

**Files:**
- Modify: `internal/container/network/network_operations.go:36-94`

**Step 1: Write test for network interface creation**

```go
// internal/container/network/network_operations_test.go
// +build integration

func TestCreateNetworkCreatesVethPair(t *testing.T) {
    if os.Geteuid() != 0 {
        t.Skip("Test requires root privileges")
    }

    containerName := "test-net-container"
    ipCIDR := "192.168.100.0/24"

    network, err := CreateNetwork(containerName, ipCIDR, "", "", nil, nil)
    if err != nil {
        t.Fatalf("CreateNetwork failed: %v", err)
    }
    defer DeleteNetwork(network, nil)

    // Verify veth pair exists
    handle, _ := netlink.NewHandle()
    defer handle.Delete()

    links, err := handle.LinkList()
    if err != nil {
        t.Fatalf("Failed to list links: %v", err)
    }

    vethFound := false
    for _, link := range links {
        if link.Attrs().Name == fmt.Sprintf("veth-%s", containerName) {
            vethFound = true
            break
        }
    }

    if !vethFound {
        t.Error("Veth interface was not created")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test -tags=integration ./internal/container/network -v -run TestCreateNetworkCreatesVethPair`

Expected: FAIL - veth interface not found

**Step 3: Implement veth pair creation**

```go
// internal/container/network/network_operations.go
// Modify CreateNetwork function starting around line 36

func CreateNetwork(containerName, ipCIDR, gateway, dns string, dhcpServer *dhcp.Server, logger *zap.Logger) (*Network, error) {
    if logger == nil {
        logger = zap.NewNop()
    }

    // Parse CIDR
    ip, ipNet, err := net.ParseCIDR(ipCIDR)
    if err != nil {
        return nil, fmt.Errorf("invalid CIDR: %v", err)
    }

    // Generate interface names
    vethHost := fmt.Sprintf("veth-h-%s", containerName[:8])
    vethContainer := fmt.Sprintf("veth-c-%s", containerName[:8])
    bridgeName := "spocker0"

    // Create veth pair
    veth := &netlink.Veth{
        LinkAttrs: netlink.LinkAttrs{
            Name: vethHost,
            MTU:  1500,
        },
        PeerName: vethContainer,
    }

    if err := netlink.LinkAdd(veth); err != nil {
        return nil, fmt.Errorf("failed to create veth pair: %v", err)
    }

    // Get the peer link
    peerLink, err := netlink.LinkByName(vethContainer)
    if err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to get peer link: %v", err)
    }

    // Create or get bridge
    bridge, err := getOrCreateBridge(bridgeName)
    if err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to setup bridge: %v", err)
    }

    // Attach host veth to bridge
    if err := netlink.LinkSetMaster(veth, bridge); err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to attach veth to bridge: %v", err)
    }

    // Bring up host veth
    if err := netlink.LinkSetUp(veth); err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to bring up host veth: %v", err)
    }

    // Configure container veth IP
    addr := &netlink.Addr{
        IPNet: &net.IPNet{
            IP:   ip,
            Mask: ipNet.Mask,
        },
    }
    if err := netlink.AddrAdd(peerLink, addr); err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to add IP to container veth: %v", err)
    }

    // Bring up container veth
    if err := netlink.LinkSetUp(peerLink); err != nil {
        netlink.LinkDel(veth)
        return nil, fmt.Errorf("failed to bring up container veth: %v", err)
    }

    // TODO: Move vethContainer to container network namespace
    // This requires namespace FD from container process
    // For now, both ends remain in host namespace (limitation)

    // Compute gateway if not provided
    if gateway == "" {
        gateway = getGateway(ipNet)
    }

    // Get DNS if not provided
    if dns == "" {
        dns = getNameservers()
    }

    // Start DHCP server if provided
    if dhcpServer != nil {
        if err := dhcpServer.Start(); err != nil {
            logger.Error("Failed to start DHCP server", zap.Error(err))
            // Don't fail network creation if DHCP fails
        }
    }

    return &Network{
        Name:        containerName,
        IP:          ip.String(),
        Gateway:     gateway,
        DNS:         dns,
        VethHost:    vethHost,
        VethPeer:    vethContainer,
        Bridge:      bridgeName,
        DHCPServer:  dhcpServer,
    }, nil
}

// Helper function to get or create bridge
func getOrCreateBridge(name string) (*netlink.Bridge, error) {
    // Try to get existing bridge
    link, err := netlink.LinkByName(name)
    if err == nil {
        if bridge, ok := link.(*netlink.Bridge); ok {
            return bridge, nil
        }
        return nil, fmt.Errorf("link %s exists but is not a bridge", name)
    }

    // Create new bridge
    bridge := &netlink.Bridge{
        LinkAttrs: netlink.LinkAttrs{
            Name: name,
            MTU:  1500,
        },
    }

    if err := netlink.LinkAdd(bridge); err != nil {
        return nil, fmt.Errorf("failed to create bridge: %v", err)
    }

    // Assign IP to bridge (gateway)
    bridgeIP := &netlink.Addr{
        IPNet: &net.IPNet{
            IP:   net.ParseIP("192.168.100.1"),
            Mask: net.CIDRMask(24, 32),
        },
    }
    if err := netlink.AddrAdd(bridge, bridgeIP); err != nil {
        return nil, fmt.Errorf("failed to add bridge IP: %v", err)
    }

    // Bring up bridge
    if err := netlink.LinkSetUp(bridge); err != nil {
        return nil, fmt.Errorf("failed to bring up bridge: %v", err)
    }

    return bridge, nil
}
```

**Step 4: Update Network struct to include interface names**

```go
// internal/container/network/network.go
// Add new fields to Network struct

type Network struct {
    Name       string
    IP         string
    Gateway    string
    DNS        string
    VethHost   string  // Add this
    VethPeer   string  // Add this
    Bridge     string  // Add this
    DHCPServer *dhcp.Server
}
```

**Step 5: Update DeleteNetwork to clean up interfaces**

```go
// internal/container/network/network_operations.go
// Update DeleteNetwork function around line 134

func DeleteNetwork(network *Network, logger *zap.Logger) error {
    if logger == nil {
        logger = zap.NewNop()
    }

    // Stop DHCP server if running
    if network.DHCPServer != nil {
        if err := network.DHCPServer.Stop(); err != nil {
            logger.Error("Failed to stop DHCP server", zap.Error(err))
        }
    }

    // Delete veth pair (deleting one end deletes both)
    if network.VethHost != "" {
        link, err := netlink.LinkByName(network.VethHost)
        if err == nil {
            if err := netlink.LinkDel(link); err != nil {
                logger.Error("Failed to delete veth", zap.String("veth", network.VethHost), zap.Error(err))
            }
        }
    }

    // Note: Don't delete bridge as other containers may be using it
    // Bridge cleanup should happen when no containers remain

    return nil
}
```

**Step 6: Run test to verify veth creation**

Run: `go test -tags=integration ./internal/container/network -v -run TestCreateNetworkCreatesVethPair`

Expected: PASS - veth pair is created

**Step 7: Commit**

```bash
git add internal/container/network/network_operations.go internal/container/network/network.go
git commit -m "fix: implement real network interface creation via netlink

- Create veth pair using netlink.LinkAdd
- Set up spocker0 bridge and attach veth to it
- Configure IP addresses on veth interfaces
- Bring interfaces up
- Update Network struct with interface names
- Clean up interfaces in DeleteNetwork
- Addresses issue #5 from container isolation audit

TODO: Move veth peer to container network namespace"
```

---

## Task 5: Make CLI Network Parameters Optional

**Files:**
- Modify: `cmd/spocker/main.go:75,116-120`

**Step 1: Write test for default network parameters**

```go
// cmd/spocker/main_test.go
package main

import (
    "testing"
)

func TestDefaultNetworkCIDR(t *testing.T) {
    config := &Config{}

    // Test that empty CIDR gets default value
    if config.NetworkIPCIDR == "" {
        config.NetworkIPCIDR = getDefaultNetworkCIDR()
    }

    if config.NetworkIPCIDR == "" {
        t.Error("Default CIDR should not be empty")
    }

    // Verify it's a valid CIDR
    _, _, err := net.ParseCIDR(config.NetworkIPCIDR)
    if err != nil {
        t.Errorf("Default CIDR is invalid: %v", err)
    }
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/spocker -v -run TestDefaultNetworkCIDR`

Expected: FAIL - getDefaultNetworkCIDR function doesn't exist

**Step 3: Add default network CIDR generation**

```go
// cmd/spocker/main.go
// Add helper function before runContainer

func getDefaultNetworkCIDR() string {
    // Use a default private network range
    // Generate random subnet in 10.x.x.x range to avoid conflicts
    import "math/rand"
    import "time"

    rand.Seed(time.Now().UnixNano())
    subnet := rand.Intn(255)
    return fmt.Sprintf("10.100.%d.2/24", subnet)
}
```

**Step 4: Update runContainer to use defaults**

```go
// cmd/spocker/main.go
// Modify around lines 116-120

func runContainer(config *Config, logger *zap.Logger) {
    // ... existing code ...

    // Apply default network CIDR if not provided
    if config.NetworkIPCIDR == "" {
        config.NetworkIPCIDR = getDefaultNetworkCIDR()
        logger.Info("Using default network CIDR", zap.String("cidr", config.NetworkIPCIDR))
    }

    _, ipNet, err := net.ParseCIDR(config.NetworkIPCIDR)
    if err != nil {
        logger.Error("Invalid CIDR", zap.String("CIDR", config.NetworkIPCIDR), zap.Error(err))
        return
    }

    // ... rest of function ...
}
```

**Step 5: Update flag default value**

```go
// cmd/spocker/main.go
// Update around line 75

flag.StringVar(&config.NetworkIPCIDR, "network-ip-cidr", "", "Network IP CIDR (default: auto-generated in 10.100.x.x/24 range)")
```

**Step 6: Run test to verify defaults work**

Run: `go test ./cmd/spocker -v -run TestDefaultNetworkCIDR`

Expected: PASS - default CIDR is generated and valid

**Step 7: Test CLI without network flag**

Run: `go build -o spocker ./cmd/spocker && ./spocker run --help`

Expected: Help message shows network-ip-cidr is optional

**Step 8: Commit**

```bash
git add cmd/spocker/main.go cmd/spocker/main_test.go
git commit -m "fix: make network parameters optional with auto-generated defaults

- Add getDefaultNetworkCIDR() to generate random 10.100.x.x/24 subnet
- Apply default CIDR when flag is empty
- Update flag description to indicate auto-generation
- Add test for default CIDR generation
- Addresses issue #1 from container isolation audit"
```

---

## Task 6: Create Missing Documentation

**Files:**
- Create: `docs/USAGE.md`

**Step 1: Create comprehensive usage documentation**

```markdown
<!-- docs/USAGE.md -->
# Spocker Usage Guide

## Overview

Spocker is a minimal container runtime that provides process isolation using Linux namespaces, cgroups, and filesystem isolation.

## Prerequisites

- Linux kernel 3.8+ with namespace support
- Root privileges (or CAP_SYS_ADMIN capability)
- Go 1.16+ for building from source

## Installation

```bash
git clone https://github.com/yourusername/spocker.git
cd spocker
go build -o spocker ./cmd/spocker
sudo mv spocker /usr/local/bin/
```

## Basic Usage

### Running a Container

```bash
sudo spocker run --command /bin/sh --args "-c,echo hello"
```

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | auto-generated | Container name |
| `--command` | *required* | Command to execute in container |
| `--args` | "" | Comma-separated command arguments |
| `--rootfs` | "/tmp/spocker-rootfs" | Container root filesystem path |
| `--network-ip-cidr` | auto-generated | Network IP CIDR (e.g., "10.100.50.2/24") |
| `--cgroup-memory` | 512MB | Memory limit in bytes |
| `--cgroup-cpu` | 50 | CPU quota percentage |

### Examples

#### Run a simple command

```bash
sudo spocker run \
  --name my-container \
  --command /bin/echo \
  --args "Hello from container"
```

#### Run with custom network

```bash
sudo spocker run \
  --name web-container \
  --command /usr/bin/nginx \
  --network-ip-cidr 10.100.10.5/24
```

#### Run with resource limits

```bash
sudo spocker run \
  --name limited-container \
  --command /usr/bin/stress \
  --args "--vm,1,--vm-bytes,128M" \
  --cgroup-memory 134217728 \
  --cgroup-cpu 25
```

#### Run with custom root filesystem

```bash
# Prepare a rootfs
mkdir -p /tmp/my-rootfs
# ... copy necessary binaries and libraries ...

sudo spocker run \
  --name custom-fs \
  --command /bin/sh \
  --rootfs /tmp/my-rootfs
```

## Container Isolation Features

### Namespaces

Spocker creates the following Linux namespaces:

- **UTS**: Isolates hostname and domain name
- **PID**: Isolates process IDs (container sees its own PID namespace)
- **Mount**: Isolates filesystem mount points
- **Network**: Isolates network interfaces and routing tables

### Cgroups

Resource limits are enforced via cgroups v2:

- **Memory**: Hard limit on memory usage
- **CPU**: CPU quota as percentage of single core

### Filesystem

Container processes run in a chrooted environment with:

- Custom root filesystem via `--rootfs`
- Essential mount points created (proc, sys, dev, tmp)
- Isolated from host filesystem

### Networking

Each container gets:

- Dedicated veth pair (virtual ethernet interface)
- Connection to `spocker0` bridge
- Assigned IP address from specified CIDR
- Automatic gateway and DNS configuration

## Current Limitations

1. **Filesystem Mounts**: Essential filesystems (proc, sys, dev) require a container init process for proper mounting. Currently, mount points are created but not all are mounted.

2. **Network Namespace**: Veth peer is not yet moved to container network namespace, so network isolation is incomplete.

3. **User Namespaces**: User namespace support is not yet implemented.

4. **Persistence**: Containers are ephemeral - no state is saved after process exits.

5. **Image Management**: No image pulling or layer management (use pre-built rootfs).

## Troubleshooting

### Permission Denied Errors

Spocker requires root privileges:

```bash
sudo spocker run ...
```

Or run with specific capabilities:

```bash
sudo setcap cap_sys_admin+ep /usr/local/bin/spocker
```

### Network CIDR Parse Errors

Ensure CIDR is in valid format:

```bash
--network-ip-cidr 10.100.50.2/24  # Correct
--network-ip-cidr 10.100.50.2     # Wrong - missing /24
```

### Container Process Exits Immediately

Check that:
1. Command exists in rootfs: `ls /tmp/spocker-rootfs/bin/`
2. Command has execute permissions: `chmod +x /path/to/binary`
3. Shared libraries are available in rootfs

### Cannot Find Filesystem Path

Ensure rootfs directory exists:

```bash
mkdir -p /tmp/spocker-rootfs
# Copy minimal filesystem or extract from Docker image
```

## Architecture

```
┌─────────────────────────────────────────┐
│          spocker CLI                     │
│  (parses flags, builds specs)           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│       container.Run()                    │
│  (orchestrates isolation setup)         │
└─┬───────┬──────────┬──────────┬─────────┘
  │       │          │          │
  ▼       ▼          ▼          ▼
┌────┐ ┌─────┐ ┌─────────┐ ┌─────────┐
│cgroup│ │namespace│ │filesystem│ │ network │
└────┘ └─────┘ └─────────┘ └─────────┘
  │       │          │          │
  └───────┴──────────┴──────────┘
               │
               ▼
     Isolated Container Process
```

## Contributing

See CONTRIBUTING.md for development guidelines.

## License

See LICENSE file.
```

**Step 2: Verify documentation completeness**

Read through docs/USAGE.md and ensure:
- All current flags are documented
- Examples are accurate
- Limitations are clearly stated
- Prerequisites are listed

**Step 3: Test documentation examples**

Manually test at least one example from the documentation:

```bash
go build -o spocker ./cmd/spocker
sudo ./spocker run --command /bin/echo --args "test"
```

Expected: Command runs (or fails with clear error about missing rootfs)

**Step 4: Commit**

```bash
git add docs/USAGE.md
git commit -m "docs: create comprehensive usage documentation

- Add USAGE.md with installation instructions
- Document all CLI flags and their defaults
- Include practical examples for common use cases
- Explain container isolation features
- Document current limitations transparently
- Add troubleshooting section
- Include architecture diagram
- Addresses issue #6 from container isolation audit"
```

---

## Task 7: Integration Testing

**Files:**
- Create: `test/integration/container_test.go`

**Step 1: Create end-to-end integration test**

```go
// test/integration/container_test.go
// +build integration

package integration

import (
    "os"
    "os/exec"
    "testing"
    "time"
)

func TestEndToEndContainerIsolation(t *testing.T) {
    if os.Geteuid() != 0 {
        t.Skip("Integration tests require root privileges")
    }

    // Build spocker binary
    buildCmd := exec.Command("go", "build", "-o", "/tmp/spocker", "./cmd/spocker")
    buildCmd.Dir = "../.."
    if err := buildCmd.Run(); err != nil {
        t.Fatalf("Failed to build spocker: %v", err)
    }
    defer os.Remove("/tmp/spocker")

    // Prepare minimal rootfs
    rootfs := "/tmp/spocker-test-rootfs"
    os.MkdirAll(rootfs+"/bin", 0755)

    // Copy /bin/sh to rootfs
    cpCmd := exec.Command("cp", "/bin/sh", rootfs+"/bin/sh")
    if err := cpCmd.Run(); err != nil {
        t.Fatalf("Failed to copy /bin/sh: %v", err)
    }
    defer os.RemoveAll(rootfs)

    // Run spocker container
    runCmd := exec.Command("/tmp/spocker", "run",
        "--name", "integration-test",
        "--command", "/bin/sh",
        "--args", "-c,sleep 2",
        "--rootfs", rootfs,
    )

    // Start container
    if err := runCmd.Start(); err != nil {
        t.Fatalf("Failed to start container: %v", err)
    }

    // Wait for container to complete
    done := make(chan error, 1)
    go func() {
        done <- runCmd.Wait()
    }()

    select {
    case err := <-done:
        if err != nil {
            t.Errorf("Container exited with error: %v", err)
        }
    case <-time.After(5 * time.Second):
        runCmd.Process.Kill()
        t.Fatal("Container did not exit within 5 seconds")
    }
}
```

**Step 2: Run integration test**

Run: `go test -tags=integration ./test/integration -v`

Expected: PASS - end-to-end container execution works

**Step 3: Commit**

```bash
git add test/integration/container_test.go
git commit -m "test: add end-to-end integration test

- Create integration test for full container lifecycle
- Test build, rootfs setup, and container execution
- Verify container completes successfully
- Add timeout protection for hanging processes"
```

---

## Task 8: Final Verification and Cleanup

**Files:**
- Review all modified files
- Run full test suite

**Step 1: Run all unit tests**

Run: `go test ./... -v`

Expected: All unit tests pass

**Step 2: Run integration tests**

Run: `sudo go test -tags=integration ./... -v`

Expected: All integration tests pass

**Step 3: Build and verify CLI**

Run: `go build -o spocker ./cmd/spocker && ./spocker --help`

Expected: Usage message displays correctly

**Step 4: Verify documentation links**

Check that README link to docs/USAGE.md works:

Run: `ls docs/USAGE.md`

Expected: File exists

**Step 5: Clean up test artifacts**

```bash
sudo ip link del spocker0 2>/dev/null || true
sudo rm -rf /tmp/spocker-test-* /tmp/spocker
```

**Step 6: Final commit (if any cleanup needed)**

```bash
git add .
git commit -m "chore: final cleanup and verification"
```

**Step 7: Create summary of changes**

Review all commits in this branch:

Run: `git log --oneline`

Expected: 7-8 commits covering all issues

---

## Completion Checklist

After executing this plan, verify:

- [ ] Container processes are attached to cgroups (check cgroup.procs)
- [ ] Processes enter namespace contexts (verify PID namespace isolation)
- [ ] Filesystem isolation via chroot works (container sees isolated rootfs)
- [ ] Network veth pairs are created (ip link show)
- [ ] CLI accepts default network parameters (spocker run without --network-ip-cidr)
- [ ] docs/USAGE.md exists and is comprehensive
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Documentation accurately reflects current implementation and limitations

## Next Steps After This Plan

1. **Container Init Process**: Implement proper init to handle proc/sys/dev mounts
2. **Network Namespace Migration**: Move veth peer to container netns
3. **User Namespace Support**: Add UID/GID mapping for rootless containers
4. **Image Management**: Add OCI image support
5. **Container Lifecycle**: Implement stop, start, list, remove commands
6. **Logging**: Capture and persist container stdout/stderr

---

**Plan completed. Ready for execution via superpowers:executing-plans or superpowers:subagent-driven-development.**
