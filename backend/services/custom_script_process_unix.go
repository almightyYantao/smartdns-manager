//go:build linux || darwin

package services

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// 为脚本建立独立进程组，取消时一并结束普通子进程。
func configureLocalScriptProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}
