package main

import (
	"os"
	"os/exec"
	"syscall"
)

func reexec() {
	exe, err := os.Executable()
	if err == nil {
		cmd := exec.Command(exe, "start")
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
		cmd.Start()
	}
}
