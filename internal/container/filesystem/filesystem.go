package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = zap.NewProduction()
}

func NewFilesystem(config *Config) (*Filesystem, error) {
	if config.Type != "mount" && config.Type != "overlay" {
		return nil, fmt.Errorf("unsupported filesystem type: %s", config.Type)
	}

	fileInfo, err := os.Stat(config.Root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("root directory does not exist: %s", config.Root)
		}
		return nil, fmt.Errorf("failed to get file info for root directory %s: %w", config.Root, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("root path %s is not a directory", config.Root)
	}

	fs := &Filesystem{Config: *config}
	return fs, nil
}

func (fs *Filesystem) Setup() error {
	if fs.Config.Type == "mount" {
		if err := fs.mountFilesystem(); err != nil {
			return fmt.Errorf("failed to mount filesystem: %w", err)
		}
	} else if fs.Config.Type == "overlay" {
		if err := fs.createOverlayFilesystem(); err != nil {
			return fmt.Errorf("failed to create overlay filesystem: %w", err)
		}
	}

	if err := fs.applyPermissions(); err != nil {
		return fmt.Errorf("failed to apply filesystem permissions: %w", err)
	}

	return nil
}

func (fs *Filesystem) mountFilesystem() error {
	for _, mount := range fs.Config.Mounts {
		if err := fs.Mount(&mount); err != nil {
			return err
		}
	}
	return nil
}

func (fs *Filesystem) createOverlayFilesystem() error {
	// Implement overlay filesystem creation logic here
	return nil
}

func (fs *Filesystem) applyPermissions() error {
	for _, perm := range fs.Config.Permissions {
		absPath, err := fs.GetAbsolutePath(perm.Path)
		if err != nil {
			return err
		}

		if err := os.Chmod(absPath, perm.Mode); err != nil {
			return fmt.Errorf("failed to set permissions for %s: %w", perm.Path, err)
		}

		if err := os.Chown(absPath, perm.UID, perm.GID); err != nil {
			return fmt.Errorf("failed to set ownership for %s: %w", perm.Path, err)
		}
	}
	return nil
}

// Mount mounts the given mount into the filesystem.
func (fs *Filesystem) Mount(mount *Mount) error {
	err := syscall.Mount(mount.Source, filepath.Join(fs.Root, mount.Target), mount.FSType, mount.Flags, "")
	if err != nil {
		return fmt.Errorf("failed to mount %s: %w", mount.Target, err)
	}
	return nil
}

// Unmount unmounts the given mount from the filesystem.
func (fs *Filesystem) Unmount(target string) error {
	err := syscall.Unmount(filepath.Join(fs.Root, target), 0)
	if err != nil {
		return fmt.Errorf("failed to unmount %s: %w", target, err)
	}
	return nil
}

// CreateDir creates a directory in the filesystem.
func (fs *Filesystem) CreateDir(path string) error {
	err := os.MkdirAll(filepath.Join(fs.Root, path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// RemoveDir removes a directory from the filesystem.
func (fs *Filesystem) RemoveDir(path string) error {
	err := os.RemoveAll(filepath.Join(fs.Root, path))
	if err != nil {
		return fmt.Errorf("failed to remove directory %s: %w", path, err)
	}
	return nil
}

// CreateFile creates a file in the filesystem.
func (fs *Filesystem) CreateFile(path string) (*os.File, error) {
	file, err := os.Create(filepath.Join(fs.Root, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %w", path, err)
	}
	return file, nil
}

// RemoveFile removes a file from the filesystem.
func (fs *Filesystem) RemoveFile(path string) error {
	err := os.Remove(filepath.Join(fs.Root, path))
	if err != nil {
		return fmt.Errorf("failed to remove file %s: %w", path, err)
	}
	return nil
}

// CopyFile copies a file from src to dst in the filesystem.
func (fs *Filesystem) CopyFile(src string, dst string) error {
	srcPath := filepath.Join(fs.Root, src)
	dstPath := filepath.Join(fs.Root, dst)

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer func() {
		if err := srcFile.Close(); err != nil {
			logger.Error("Failed to close source file", zap.String("src", src), zap.Error(err))
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", src, err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source %s is a directory, not a file", src)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			logger.Error("Failed to close destination file", zap.String("dst", dst), zap.Error(err))
		}
	}()

	dstInfo, err := dstFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat destination file %s: %w", dst, err)
	}
	if dstInfo.IsDir() {
		return fmt.Errorf("destination %s is a directory, not a file", dst)
	}

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", src, dst, err)
	}

	return nil
}

// SetFileOwnership sets the ownership of a file in the filesystem.
func (fs *Filesystem) SetFileOwnership(path string, uid int, gid int) error {
	err := os.Chown(filepath.Join(fs.Root, path), uid, gid)
	if err != nil {
		return fmt.Errorf("failed to set ownership for file %s: %w", path, err)
	}
	return nil
}

// SetFilePermissions sets the permissions of a file in the filesystem.
func (fs *Filesystem) SetFilePermissions(path string, mode os.FileMode) error {
	err := os.Chmod(filepath.Join(fs.Root, path), mode)
	if err != nil {
		return fmt.Errorf("failed to set permissions for file %s: %w", path, err)
	}
	return nil
}

// GetAbsolutePath returns the absolute path of the given path within the filesystem.
func (fs *Filesystem) GetAbsolutePath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(fs.Root, path))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}
	return absPath, nil
}

type Filesystem struct {
	Config Config
	Root   string
}