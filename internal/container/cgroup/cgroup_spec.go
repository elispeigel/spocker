package cgroup

// Spec represents the specification for a Linux control group.
// It contains the name of the cgroup, resources to be allocated, and the root path to the cgroup.
type Spec struct {
	Name       string
	Resources  *Resources
	CgroupRoot string
}

// Resources struct contains the resource allocations for a Linux control group.
// It has fields for memory, CPU, and block I/O resources.
type Resources struct {
	Memory *Memory
	CPU    *CPU
	BlkIO  *BlkIO
}

// CPU struct represents the CPU resource allocation for a Linux control group.
// It contains a field for CPU shares.
type CPU struct {
	Shares int
}

// BlkIO struct represents the block I/O resource allocation for a Linux control group.
// It contains a field for block I/O weight.
type BlkIO struct {
	Weight int
}

// Memory struct represents the memory resource allocation for a Linux control group.
// It contains a field for memory limit.
type Memory struct {
	Limit int
}

// SpecBuilder is a builder for Spec objects.
type SpecBuilder struct {
	spec *Spec
}

// NewSpecBuilder creates a new SpecBuilder.
func NewSpecBuilder() *SpecBuilder {
	return &SpecBuilder{
		spec: &Spec{},
	}
}

// WithName sets the name of the cgroup spec.
func (b *SpecBuilder) WithName(name string) *SpecBuilder {
	b.spec.Name = name
	return b
}

// WithResources sets the resources of the cgroup spec.
func (b *SpecBuilder) WithResources(resources *Resources) *SpecBuilder {
	b.spec.Resources = resources
	return b
}

// WithCgroupRoot sets the cgroup root of the cgroup spec.
func (b *SpecBuilder) WithCgroupRoot(cgroupRoot string) *SpecBuilder {
	b.spec.CgroupRoot = cgroupRoot
	return b
}

// Build constructs the CgroupSpec object using the provided settings.
func (b *SpecBuilder) Build() *Spec {
	return b.spec
}
