package seccomp

import (
	"fmt"

	seccomp "github.com/elastic/go-seccomp-bpf"
)

func SetupSeccomp(spec *SeccompSpec) error {
    filter := seccomp.Filter{
        NoNewPrivs: true,
        Flag:       seccomp.FilterFlagTSync,
        Policy: seccomp.Policy{
            DefaultAction: seccomp.ActionAllow,
            Syscalls: []seccomp.SyscallGroup{
                {
                    Action: seccomp.ActionErrno,
                    Names: []string{
                        "fork",
                        "vfork",
                        "execve",
                        "execveat",
                    },
                },
            },
        },
    }

    for _, syscallName := range spec.AllowedSyscalls {
        filter.Policy.Syscalls = append(filter.Policy.Syscalls, seccomp.SyscallGroup{
            Action: seccomp.ActionAllow,
            Names: []string{syscallName},
        })
    }

    if err := seccomp.LoadFilter(filter); err != nil {
        return fmt.Errorf("failed to load seccomp filter: %w", err)
    }

    fmt.Println("Seccomp filter applied successfully")

    return nil
}

type SeccompSpec struct {
	AllowedSyscalls []string
}
