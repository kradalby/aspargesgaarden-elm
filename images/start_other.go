//go:build !linux

package images

import "os/exec"

// start starts cmd; only Linux has ambient capabilities to drop.
func start(cmd *exec.Cmd) error { return cmd.Start() }
