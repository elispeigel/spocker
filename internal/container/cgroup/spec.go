// internal/container/cgroup/spec.go
package cgroup

type Spec struct {
	Name        string
	Resources   *Resources
	CgroupRoot  string
}

type Resources struct {
	Memory *Memory
	CPU    *CPU
	BlkIO  *BlkIO
}

type CPU struct {
	Shares int
}

type BlkIO struct {
	Weight int
}

type Memory struct {
	Limit int
}
