//go:build !windows

package launcher

import "os/exec"

func noWindow(cmd *exec.Cmd) {}
