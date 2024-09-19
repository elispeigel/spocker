package seccomp

import (
	"fmt"
	seccomp "github.com/elastic/go-seccomp-bpf"
	"go.uber.org/zap"
)

// DefaultAllowedSyscalls is a list of syscalls that are generally safe to allow
var DefaultAllowedSyscalls = []string{
	"read", "write", "open", "close", "stat", "fstat", "lstat", "poll",
	"lseek", "mmap", "mprotect", "munmap", "brk", "rt_sigaction",
	"rt_sigprocmask", "rt_sigreturn", "ioctl", "pread64", "pwrite64",
	"readv", "writev", "access", "pipe", "select", "sched_yield",
	"mremap", "msync", "mincore", "madvise", "shmget", "shmat", "shmctl",
	"dup", "dup2", "pause", "nanosleep", "getitimer", "alarm", "setitimer",
	"getpid", "sendfile", "socket", "connect", "accept", "sendto", "recvfrom",
	"sendmsg", "recvmsg", "shutdown", "bind", "listen", "getsockname",
	"getpeername", "socketpair", "setsockopt", "getsockopt", "clone",
	"exit", "wait4", "kill", "uname", "fcntl", "flock", "fsync", "fdatasync",
	"truncate", "ftruncate", "getdents", "getcwd", "chdir", "fchdir", "rename",
	"mkdir", "rmdir", "creat", "link", "unlink", "symlink", "readlink",
	"chmod", "fchmod", "chown", "fchown", "lchown", "umask", "gettimeofday",
	"getrlimit", "getrusage", "sysinfo", "times", "ptrace", "getuid",
	"syslog", "getgid", "setuid", "setgid", "geteuid", "getegid", "setpgid",
	"getppid", "getpgrp", "setsid",
}

func SetupSeccomp(spec *SeccompSpec) error {
	logger := zap.L()

	filter := seccomp.Filter{
		NoNewPrivs: true,
		Flag:       seccomp.FilterFlagTSync,
		Policy: seccomp.Policy{
			DefaultAction: seccomp.ActionErrno,
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

	allowedSyscalls := spec.AllowedSyscalls
	if len(allowedSyscalls) == 0 {
		allowedSyscalls = DefaultAllowedSyscalls
	}

	for _, syscallName := range allowedSyscalls {
		filter.Policy.Syscalls = append(filter.Policy.Syscalls, seccomp.SyscallGroup{
			Action: seccomp.ActionAllow,
			Names:  []string{syscallName},
		})
	}

	logger.Debug("Loading seccomp filter", zap.Strings("allowed_syscalls", allowedSyscalls))
	if err := seccomp.LoadFilter(filter); err != nil {
		logger.Error("Failed to load seccomp filter", zap.Error(err))
		return fmt.Errorf("failed to load seccomp filter: %w", err)
	}

	logger.Info("Seccomp filter applied successfully")
	return nil
}

type SeccompSpec struct {
	AllowedSyscalls []string
}