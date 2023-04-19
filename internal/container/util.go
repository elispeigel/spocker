// Package container provides functions for creating a container.
package container

import (
	"context"
	"fmt"
	"os/exec"
)



// AllowedCommands is a list of allowed commands.
var AllowedCommands = map[string]bool{
	"ls":             true,
	"echo":           true,
	"mkdir":          true,
	"/bin/sh":        true,
	"/proc/self/exe": true,
}

// isCommandAllowed checks if the given command is in the allowed list.
func isCommandAllowed(cmd string) bool {
	return AllowedCommands[cmd]
}

// createCommand creates a new exec.Cmd object for the specified command and its arguments, with the given context.
func createCommand(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	if !isCommandAllowed(name) {
		return nil, fmt.Errorf("invalid command: %s", name)
	}

	cmd := exec.CommandContext(ctx, name, args...)
	return cmd, nil
}
