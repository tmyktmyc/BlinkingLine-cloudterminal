//go:build !windows

package session

import (
	"os/exec"
	"syscall"
)

type ProcState struct{}

func configureProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func configureProcCancel(cmd *exec.Cmd, ps *ProcState) {
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
}

func afterStart(cmd *exec.Cmd, ps *ProcState) error {
	return nil
}

func cleanupProc(ps *ProcState) {}
