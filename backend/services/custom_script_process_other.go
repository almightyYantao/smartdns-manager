//go:build !linux && !darwin

package services

import "os/exec"

// 其他平台保留 CommandContext 的直接进程取消语义。
func configureLocalScriptProcess(cmd *exec.Cmd) {}
