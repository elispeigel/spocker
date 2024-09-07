package filesystem

import "os"

type Config struct {
	Type        string
	Root        string
	Mounts      []Mount
	Permissions []Permission
}

type Mount struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
}

type Permission struct {
	Path string
	Mode os.FileMode
	UID  int
	GID  int
}