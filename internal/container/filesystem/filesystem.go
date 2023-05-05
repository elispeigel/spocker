package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"go.uber.org/zap"
)

var logger, _ = zap.NewProduction()

// Mount is a struct representing a mount in the container's filesystem.
type Mount struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
}

// Filesystem is an abstraction over a container's filesystem.
type Filesystem struct {
	Root string
}

type FilesystemHandler interface {
    Stat(name string) (os.FileInfo, error)
    Create(name string) (*os.File, error)
    Remove(name string) error
}

// NewFilesystem creates a new filesystem object for the given root directory.
func NewFilesystem(root string) (*Filesystem, error) {
	fileInfo, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("root directory does not exist: %s", root)
		}
		return nil, fmt.Errorf("failed to get file info for root directory: %s: %v", root, err)
	}
	if !fileInfo.IsDir() {
		return nil, fmt.Errorf("root directory is a file and not a directory: %s", root)
	}

	// Create new Filesystem object with Root field set to root directory path
	fs := &Filesystem{Root: root}
	return fs, nil
}

// Mount mounts the given mount into the filesystem.
func (fs *Filesystem) Mount(mount *Mount) error {
	err := syscall.Mount(mount.Source, filepath.Join(fs.Root, mount.Target), mount.FSType, mount.Flags, "")
	if err != nil {
		return fmt.Errorf("failed to mount %s: %v", mount.Target, err)
	}
	return nil
}

// Unmount unmounts the given mount from the filesystem.
func (fs *Filesystem) Unmount(target string) error {
	err := syscall.Unmount(filepath.Join(fs.Root, target), 0)
	if err != nil {
		return fmt.Errorf("failed to unmount %s: %v", target, err)
	}
	return nil
}

// CreateDir creates a directory in the filesystem.
func (fs *Filesystem) CreateDir(path string) error {
	err := os.MkdirAll(filepath.Join(fs.Root, path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %v", path, err)
	}
	return nil
}

// RemoveDir removes a directory from the filesystem.
func (fs *Filesystem) RemoveDir(path string) error {
	err := os.RemoveAll(filepath.Join(fs.Root, path))
	if err != nil {
		return fmt.Errorf("failed to remove directory %s: %v", path, err)
	}
	return nil
}

// CreateFile creates a file in the filesystem.
func (fs *Filesystem) CreateFile(path string) (*os.File, error) {
	file, err := os.Create(filepath.Join(fs.Root, path))
	if err != nil {
		return nil, fmt.Errorf("failed to create file %s: %v", path, err)
	}
	return file, nil
}

// RemoveFile removes a file from the filesystem.
func (fs *Filesystem) RemoveFile(path string) error {
	err := os.Remove(filepath.Join(fs.Root, path))
	if err != nil {
		return fmt.Errorf("failed to remove file %s: %v", path, err)
	}
	return nil
}

// CopyFile copies a file from src to dst in the filesystem.
func (fs *Filesystem) CopyFile(src string, dst string) error {
	srcPath := filepath.Join(fs.Root, src)
	dstPath := filepath.Join(fs.Root, dst)

	// Open the source file for reading
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %v", src, err)
	}
	defer func() {
		errSrcClose := srcFile.Close()
		if errSrcClose != nil {
			logger.Error("failed to close source file", zap.String("src", src), zap.Error(errSrcClose))
		}
	}()

	// Check if src is a directory
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %v", src, err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("source is a directory %s", src)
	}

	// Create the destination file for writing
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %v", dst, err)
	}
	defer func() {
		errDstClose := dstFile.Close()
		if errDstClose != nil {
			logger.Error("failed to close destination file", zap.String("dst", dst), zap.Error(errDstClose))
		}
	}()

	// Check if dst is a directory
	dstInfo, err := dstFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat destination file %s: %v", dst, err)
	}
	if dstInfo.IsDir() {
		return fmt.Errorf("destination is a directory %s", dst)
	}

	// Copy the contents of the source file to the destination file
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %v", src, dst, err)
	}

	return nil
}

// SetFileOwnership sets the ownership of a file in the filesystem.
func (fs *Filesystem) SetFileOwnership(path string, uid int, gid int) error {
	err := os.Chown(filepath.Join(fs.Root, path), uid, gid)
	if err != nil {
		return fmt.Errorf("failed to set ownership for file %s: %v", path, err)
	}
	return nil
}

// SetFilePermissions sets the permissions of a file in the filesystem.
func (fs *Filesystem) SetFilePermissions(path string, mode os.FileMode) error {
	err := os.Chmod(filepath.Join(fs.Root, path), mode)
	if err != nil {
		return fmt.Errorf("failed to set permissions for file %s: %v", path, err)
	}
	return nil
}

// GetAbsolutePath returns the absolute path of the given path within the filesystem.
func (fs *Filesystem) GetAbsolutePath(path string) (string, error) {
	absPath, err := filepath.Abs(filepath.Join(fs.Root, path))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %v", path, err)
	}
	return absPath, nil
}
