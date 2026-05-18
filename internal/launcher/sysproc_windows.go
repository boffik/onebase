//go:build windows

package launcher

import (
	"os/exec"
	"syscall"
)

// noWindow sets CREATE_NO_WINDOW so the process never gets a visible console.
func noWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
