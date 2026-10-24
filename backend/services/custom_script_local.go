package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"time"
)

// 空模式保持原来的远程执行语义；本机任务不允许混入远程节点。
func scriptRunsLocally(mode string, nodeIDs []uint) (bool, error) {
	switch mode {
	case "", "remote":
		return false, nil
	case "local":
		if len(nodeIDs) != 0 {
			return false, fmt.Errorf("管理台本机任务不能同时指定远程节点")
		}
		return true, nil
	default:
		return false, fmt.Errorf("未知脚本执行模式: %s", mode)
	}
}

// 本机脚本始终使用管理服务已有的系统用户，不提升权限或切换用户。
func executeLocalScript(ctx context.Context, script, workingDir, runAsUser string,
	envVars map[string]string, timeout time.Duration) (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("无法读取管理服务用户: %w", err)
	}
	if runAsUser != "" && runAsUser != current.Username {
		return "", fmt.Errorf("本机任务只能使用管理服务用户 %s", current.Username)
	}
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	if workingDir == "" {
		workingDir = os.TempDir()
	}
	scriptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(scriptCtx, "/bin/sh", "-eu", "-c", script)
	configureLocalScriptProcess(cmd)
	cmd.WaitDelay = time.Second
	cmd.Dir = workingDir
	cmd.Env = os.Environ()
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		cmd.Env = append(cmd.Env, key+"="+envVars[key])
	}
	output, err := cmd.CombinedOutput()
	if scriptCtx.Err() != nil {
		return string(output), fmt.Errorf("管理台本机脚本超时或取消: %w", scriptCtx.Err())
	}
	if err != nil {
		return string(output), fmt.Errorf("管理台本机脚本执行失败: %w", err)
	}
	return string(output), nil
}
