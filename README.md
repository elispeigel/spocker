Container Package

The container package provides an abstraction over a Linux control group (cgroup) and allows users to limit the resources of a process. The package contains the following functions and types:
Functions
NewCgroup

NewCgroup returns a new cgroup object based on the provided CgroupSpec. It creates a cgroup directory, tracks the tasks in the cgroup, and sets the memory limit if specified in the CgroupSpec. The function returns an error if any of these steps fail.
MustLimitMemory

MustLimitMemory limits the memory usage of the current process. It creates a cgroup object and sets the memory limit control to the specified value. The function logs a fatal error if any of these steps fail.
Types
Cgroup

Cgroup is an abstraction over a Linux control group. It contains the name of the cgroup and a file that tracks the tasks in the cgroup.
CgroupSpec

CgroupSpec represents the specification for a Linux control group. It contains the name of the cgroup and the resources to limit, currently only memory resources are supported.
Resources

Resources contains the resource limits for a cgroup. Currently, only memory resources are supported.
Memory

Memory contains the memory limit for a cgroup.
Example Usage

```

import (
    "log"
    "github.com/example/container"
)

func main() {
    // Limit the memory usage of the current process to 128 MB
    container.MustLimitMemory(128 * 1024 * 1024)
    
    // ... Run the main process ...
}
```
This code limits the memory usage of the current process to 128 MB using the MustLimitMemory function provided by the container package. If any of the steps fail, a fatal error is logged.
