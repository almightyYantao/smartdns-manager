package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var coreTag = regexp.MustCompile(`^Release([0-9]{1,6})(?:\.([0-9]{1,6}))?$`)
var coreVersionTag = regexp.MustCompile(`\((Release[0-9]+(?:\.[0-9]+)?)\)`)
var coreDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var coreProbeName = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.?$`)

const coreProject = "https://github.com/pymumu/smartdns/releases/"

// CoreRelease 是官方稳定版元数据。下载地址由发布标签与受支持的资产名构造。
type CoreRelease struct {
	Tag        string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	baseURL    string
	Assets     []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func (r CoreRelease) asset(arch string) (string, string, bool) {
	if !coreTag.MatchString(r.Tag) || (arch != "x86_64" && arch != "aarch64") {
		return "", "", false
	}
	name := "smartdns-" + arch
	for _, a := range r.Assets {
		if a.Name == name && coreDigest.MatchString(a.Digest) {
			return r.baseURL + "/" + r.Tag + "/" + name, strings.TrimPrefix(a.Digest, "sha256:"), true
		}
	}
	return "", "", false
}

func compareCoreTags(a, b string) (int, bool) {
	am, bm := coreTag.FindStringSubmatch(a), coreTag.FindStringSubmatch(b)
	if am == nil || bm == nil {
		return 0, false
	}
	for i := 1; i <= 2; i++ {
		x, _ := strconv.Atoi(am[i])
		y, _ := strconv.Atoi(bm[i])
		if x != y {
			return x - y, true
		}
	}
	return 0, true
}

type CoreStatus struct {
	Version    string `json:"version"`
	Tag        string `json:"tag"`
	Arch       string `json:"arch"`
	Latest     string `json:"latest"`
	CanUpgrade bool   `json:"can_upgrade"`
	State      string `json:"state"`
}

func (c *SSHClient) DetectCore(r CoreRelease) (CoreStatus, error) {
	status := CoreStatus{Latest: r.Tag}
	output, err := c.ExecuteCommandWithTimeout(`set -eu
pid=$(systemctl show -p MainPID smartdns | cut -d= -f2)
test "$pid" -gt 0
uname -m
"/proc/$pid/exe" -v 2>&1`, 8*time.Second)
	if err != nil {
		return status, fmt.Errorf("读取运行中的 SmartDNS 版本失败")
	}
	arch, version, ok := strings.Cut(strings.TrimSpace(output), "\n")
	if !ok {
		return status, fmt.Errorf("运行版本输出不完整")
	}
	status.Arch, status.Version = strings.TrimSpace(arch), strings.TrimSpace(version)
	if m := coreVersionTag.FindStringSubmatch(version); m != nil {
		status.Tag = m[1]
	}
	comparison, known := compareCoreTags(status.Tag, r.Tag)
	_, _, assetOK := r.asset(status.Arch)
	switch {
	case r.Tag == "":
		status.State = "发布源不可用"
	case !known:
		status.State = "版本无法比较"
	case comparison >= 0:
		status.State = "已达到稳定版"
	case !assetOK:
		status.State = "缺少可校验的升级包"
	default:
		status.CanUpgrade = true
		status.State = "有新版本"
	}
	return status, nil
}

func (c *SSHClient) UpgradeCore(ctx context.Context, r CoreRelease, current CoreStatus, probeName, expectedIP string) (string, error) {
	download, digest, ok := r.asset(current.Arch)
	if !ok || !current.CanUpgrade || !coreTag.MatchString(current.Tag) || len(probeName) > 253 || !coreProbeName.MatchString(probeName) {
		return "", fmt.Errorf("升级条件校验失败")
	}
	// 用严格校验后的常量作为脚本参数，SSH 会话内负责切换和回退。
	addr, err := parseExpectedIPv4(expectedIP)
	if err != nil {
		return "", err
	}
	data, err := readCoreSource(ctx, download, maxCoreAsset, 40*time.Second)
	if err != nil {
		return "", fmt.Errorf("控制台下载升级包失败: %w", err)
	}
	if err := verifyCoreAsset(data, digest); err != nil {
		return "", err
	}
	command := "/bin/sh -s -- " + strings.Join([]string{digest, current.Tag, r.Tag, probeName, addr}, " ")
	return c.runCoreUpgrade(command, data)
}
