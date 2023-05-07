package cgroup

import "os"

type FileHandler interface {
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	ReadFile(filename string) ([]byte, error)
	MkdirAll(path string, perm os.FileMode) error
	RemoveAll(path string) error
}

type DefaultFileHandler struct{}

func (d *DefaultFileHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (d *DefaultFileHandler) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (d *DefaultFileHandler) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (d *DefaultFileHandler) RemoveAll(path string) error {
	return os.RemoveAll(path)
}
