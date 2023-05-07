// cgroup package manages Linux control groups (cgroups) and provides functionality to apply resource limitations.
package cgroup

import "os"

// OpenFile wraps os.OpenFile, opening a file with the specified name, flag, and permission mode.
func (d *DefaultFileHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// ReadFile wraps os.ReadFile, reading the content of the specified filename.
func (d *DefaultFileHandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

// MkdirAll wraps os.MkdirAll, creating a directory with the specified path and permission mode, including any necessary parents.
func (d *DefaultFileHandler) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveAll wraps os.RemoveAll, removing the specified path and its contents recursively.
func (d *DefaultFileHandler) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
