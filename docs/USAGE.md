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
