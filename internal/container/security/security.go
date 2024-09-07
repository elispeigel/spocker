package security

import (
	"fmt"
	"os"
	"syscall"

	"go.uber.org/zap"
)

func HardenConfiguration(spec *SecuritySpec) error {
	if spec.EnableNoNewPrivileges {
		if err := setNoNewPrivileges(); err != nil {
			return fmt.Errorf("failed to set no new privileges: %w", err)
		}
	}

	if spec.ReadOnlyRootFS {
		if err := setReadOnlyRootFS(spec.RootFS); err != nil {
			return fmt.Errorf("failed to set read-only rootfs: %w", err)
		}
	}

	// Add more security hardening configurations as needed

	zap.L().Info("Security configuration applied successfully")

	return nil
}

func setNoNewPrivileges() error {
	if err := os.Setenv("SECCOMP_ALLOW_PROCESS_CREATION_BY_NON_ROOT", "0"); err != nil {
		return fmt.Errorf("failed to set SECCOMP_ALLOW_PROCESS_CREATION_BY_NON_ROOT: %w", err)
	}
	return nil
}

func setReadOnlyRootFS(rootfs string) error {
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_RDONLY|syscall.MS_REMOUNT, ""); err != nil {
		return fmt.Errorf("failed to remount rootfs as read-only: %w", err)
	}
	return nil
}

type SecuritySpec struct {
	EnableNoNewPrivileges bool
	ReadOnlyRootFS        bool
	RootFS                string
}