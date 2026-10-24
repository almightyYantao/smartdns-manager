package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLocalScriptUsesWorkingDirectoryAndEnvironment(t *testing.T) {
	dir := t.TempDir()
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	output, err := executeLocalScript(context.Background(), `printf '%s' "$SYNC_TEST_VALUE"; pwd`,
		dir, "", map[string]string{"SYNC_TEST_VALUE": "working:"}, time.Second)
	if err != nil || !strings.Contains(output, "working:"+realDir) {
		t.Fatalf("未使用任务目录或环境变量: %q %v", output, err)
	}
}

func TestLocalScriptFailureKeepsOutputAndReturnsError(t *testing.T) {
	output, err := executeLocalScript(context.Background(), "echo sync-failed >&2; exit 7", "", "", nil, time.Second)
	if err == nil || !strings.Contains(output, "sync-failed") {
		t.Fatalf("失败不能被标记为成功: %q %v", output, err)
	}
}

func TestLocalScriptTimeoutStopsExecutor(t *testing.T) {
	_, err := executeLocalScript(context.Background(), "exec sleep 10", "", "", nil, 30*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("未传播任务超时: %v", err)
	}
}

func TestLocalScriptCannotSwitchUser(t *testing.T) {
	_, err := executeLocalScript(context.Background(), "true", "", "nonexistent-sync-user", nil, time.Second)
	if err == nil {
		t.Fatal("本机脚本不能切换用户")
	}
}

func TestLocalExecutionModeIsExplicit(t *testing.T) {
	for _, mode := range []string{"", "remote"} {
		if local, err := scriptRunsLocally(mode, nil); local || err != nil {
			t.Fatalf("远程任务语义发生变化: %q", mode)
		}
	}
	if local, err := scriptRunsLocally("local", nil); !local || err != nil {
		t.Fatal("本机模式不可用")
	}
	for _, test := range []struct {
		mode string
		ids  []uint
	}{{"local", []uint{1}}, {"invalid", nil}} {
		if _, err := scriptRunsLocally(test.mode, test.ids); err == nil {
			t.Fatal("无效执行模式未被拒绝")
		}
	}
}

func TestLocalScriptCanWriteInWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := executeLocalScript(context.Background(), "printf ok > result", dir, "", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "result"))
	if err != nil || string(content) != "ok" {
		t.Fatal("任务未在指定目录执行")
	}
}

func TestLocalScriptTimeoutStopsChildProcess(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("进程组取消仅适用于 Linux/macOS")
	}
	dir := t.TempDir()
	script := fmt.Sprintf("sleep 0.3; printf leaked > %q", filepath.Join(dir, "leaked"))
	start := time.Now()
	_, err := executeLocalScript(context.Background(), script, dir, "", nil, 50*time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("未传播超时: %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("子进程导致取消后继续等待")
	}
	time.Sleep(400 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(dir, "leaked")); !os.IsNotExist(err) {
		t.Fatal("超时后子进程仍然执行")
	}
}

func TestLocalScriptPropagatesParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := executeLocalScript(ctx, "echo unexpected", "", "", nil, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("未传播父任务取消: %v", err)
	}
}
