// Package container provides functions for creating a container.
package container

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestNewFilesystem(t *testing.T) {
	t.Run("valid root directory", func(t *testing.T) {
		// Create a temporary directory to use as the root directory
		rootDir, err := os.MkdirTemp("", "test_fs")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(rootDir)

		// Test that a valid root directory creates a new filesystem object
		fs, err := NewFilesystem(rootDir)
		if err != nil {
			t.Fatalf("failed to create filesystem object: %v", err)
		}

		// Test that the created filesystem object has the correct Root field
		if fs.Root != rootDir {
			t.Errorf("filesystem root is incorrect: expected %s, got %s", rootDir, fs.Root)
		}
	})

	t.Run("invalid root directory", func(t *testing.T) {
		// Create a temporary directory to use as the root directory
		rootDir, err := os.MkdirTemp("", "test_fs")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(rootDir)

		// Test that an invalid root directory returns an error
		invalidDir := filepath.Join(rootDir, "invalid")
		err = os.Mkdir(invalidDir, 0755)
		if err != nil {
			t.Fatalf("failed to create invalid root dir: %v", err)
		}
		err = os.Remove(invalidDir)
		if err != nil {
			t.Fatalf("failed to remove invalid root dir: %v", err)
		}
		_, err = NewFilesystem(invalidDir)
		if err == nil {
			t.Errorf("expected error for invalid root directory, but got nil")
		}
	})
}

func TestMountUnmount(t *testing.T) {
	t.Run("mount and unmount", func(t *testing.T) {
		// Set up temporary directory for root
		root, err := os.MkdirTemp("", "test-root")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(root)

		// Create a new Filesystem object
		fs, err := NewFilesystem(root)
		if err != nil {
			t.Fatal(err)
		}

		// Set up temporary directory for mount
		mount, err := os.MkdirTemp("", "test-mount")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(mount)

		// Create a new Mount object
		m := &Mount{
			Source: "tmpfs",
			Target: mount,
			FSType: "tmpfs",
			Flags:  syscall.MS_NOSUID,
		}

		// Mount the filesystem
		if err := fs.Mount(m); err != nil {
			t.Fatalf("failed to mount filesystem: %v", err)
		}

		// Check if the mountpoint is actually mounted
		if !isMounted(mount) {
			t.Errorf("mountpoint %s is not mounted", mount)
		}

		// Unmount the filesystem
		if err := fs.Unmount(mount); err != nil {
			t.Fatalf("failed to unmount filesystem: %v", err)
		}
		// Check if the mountpoint is actually unmounted
		if isMounted(mount) {
			t.Errorf("mountpoint %s is still mounted", mount)
		}
	})
}

// isMounted checks if the given mountpoint is currently mounted.
func isMounted(mountpoint string) bool {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		if fields[1] == mountpoint {
			return true
		}
	}

	return false
}

func TestCreateRemoveDir(t *testing.T) {
	t.Run("create and remove directory", func(t *testing.T) {
		// Set up temporary directory for filesystem
		root, err := os.MkdirTemp("", "test-filesystem")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(root)
		// Create new Filesystem object with temporary directory as root
		fs := &Filesystem{Root: root}

		// Create a directory and verify that it was created
		dirPath := "test-dir"
		if err := fs.CreateDir(dirPath); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(fs.Root, dirPath)); err != nil {
			t.Fatalf("directory not found: %v", err)
		}

		// Remove the directory and verify that it was removed
		if err := fs.RemoveDir(dirPath); err != nil {
			t.Fatalf("failed to remove directory: %v", err)
		}
		if _, err := os.Stat(filepath.Join(fs.Root, dirPath)); !os.IsNotExist(err) {
			t.Fatalf("directory still exists after removal")
		}
	})
}

func TestCreateRemoveFile(t *testing.T) {
	fs, err := NewFilesystem("/tmp")
	if err != nil {
		t.Fatalf("failed to create filesystem: %v", err)
	}

	path := "testfile.txt"

	// Test creating a new file
	file, err := fs.CreateFile(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer file.Close()

	// Check if the file exists
	if _, err := os.Stat("/tmp/" + path); err != nil {
		t.Fatalf("failed to check if file exists: %v", err)
	}

	// Test removing the file
	if err := fs.RemoveFile(path); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}

	// Check if the file was removed
	if _, err := os.Stat("/tmp/" + path); !os.IsNotExist(err) {
		t.Fatalf("file was not removed")
	}
}

func TestCopyFile(t *testing.T) {
	fs, err := NewFilesystem("/tmp")
	if err != nil {
		t.Fatalf("failed to create filesystem: %v", err)
	}

	srcPath := "testfile.txt"
	dstPath := "testfile_copy.txt"

	// Create a new file
	file, err := fs.CreateFile(srcPath)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	defer file.Close()

	// Copy the file to a new location
	if err := fs.CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("failed to copy file: %v", err)
	}

	// Check if the destination file exists
	if _, err := os.Stat("/tmp/" + dstPath); err != nil {
		t.Fatalf("failed to check if destination file exists: %v", err)
	}

	// Test removing the files
	if err := fs.RemoveFile(srcPath); err != nil {
		t.Fatalf("failed to remove source file: %v", err)
	}
	if err := fs.RemoveFile(dstPath); err != nil {
		t.Fatalf("failed to remove destination file: %v", err)
	}

	// Check if the files were removed
	if _, err := os.Stat("/tmp/" + srcPath); !os.IsNotExist(err) {
		t.Fatalf("source file was not removed")
	}
	if _, err := os.Stat("/tmp/" + dstPath); !os.IsNotExist(err) {
		t.Fatalf("destination file was not removed")
	}
}

func TestSetFileOwnership(t *testing.T) {
	// Create a temporary directory to use for the filesystem root
	rootDir, err := os.MkdirTemp("", "fs-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Create a new filesystem object
	fs, err := NewFilesystem(rootDir)
	if err != nil {
		t.Fatalf("failed to create filesystem: %v", err)
	}

	// Create a test file
	testFilePath := "testfile"
	testFile, err := fs.CreateFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	testFile.Close()

	// Set ownership of test file
	uid := 1000
	gid := 1000
	err = fs.SetFileOwnership(testFilePath, uid, gid)
	if err != nil {
		t.Errorf("failed to set file ownership: %v", err)
	}

	// Check ownership of test file
	fileInfo, err := os.Stat(filepath.Join(fs.Root, testFilePath))
	if err != nil {
		t.Errorf("failed to get file info: %v", err)
	}
	stat := fileInfo.Sys().(*syscall.Stat_t)
	if int(stat.Uid) != uid || int(stat.Gid) != gid {
		t.Errorf("file ownership not set correctly, expected uid %d and gid %d, got uid %d and gid %d",
			uid, gid, int(stat.Uid), int(stat.Gid))
	}
}

func TestSetFilePermissions(t *testing.T) {
	// Create a temporary directory to use for the filesystem root
	rootDir, err := os.MkdirTemp("", "fs-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Create a new filesystem object
	fs, err := NewFilesystem(rootDir)
	if err != nil {
		t.Fatalf("failed to create filesystem: %v", err)
	}

	// Create a test file
	testFilePath := "testfile"
	testFile, err := fs.CreateFile(testFilePath)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	testFile.Close()

	// Set permissions of test file
	permissions := os.FileMode(0644)
	err = fs.SetFilePermissions(testFilePath, permissions)
	if err != nil {
		t.Errorf("failed to set file permissions: %v", err)
	}

	// Check permissions of test file
	fileInfo, err := os.Stat(filepath.Join(fs.Root, testFilePath))
	if err != nil {
		t.Errorf("failed to get file info: %v", err)
	}
	if fileInfo.Mode().Perm() != permissions {
		t.Errorf("file permissions not set correctly, expected %s, got %s",
			permissions.String(), fileInfo.Mode().Perm().String())
	}
}

func TestGetAbsolutePath(t *testing.T) {
	// Create a new filesystem with a root directory
	fs, err := NewFilesystem("/tmp")
	if err != nil {
		t.Errorf("NewFilesystem failed with error: %v", err)
	}
	// Test that an absolute path is returned for a relative path
	absPath, err := fs.GetAbsolutePath("file.txt")
	if err != nil {
		t.Errorf("GetAbsolutePath failed with error: %v", err)
	}
	expectedPath := "/tmp/file.txt"
	if absPath != expectedPath {
		t.Errorf("GetAbsolutePath returned %s, expected %s", absPath, expectedPath)
	}

	// Test that an absolute path is returned for an absolute path
	absPath, err = fs.GetAbsolutePath("/var/log")
	if err != nil {
		t.Errorf("GetAbsolutePath failed with error: %v", err)
	}
	expectedPath = "/var/log"
	if absPath != expectedPath {
		t.Errorf("GetAbsolutePath returned %s, expected %s", absPath, expectedPath)
	}

	// Test that an error is returned for a non-existent path
	_, err = fs.GetAbsolutePath("nonexistent/file.txt")
	if err == nil {
		t.Errorf("GetAbsolutePath should have returned an error for non-existent path")
	}
}
