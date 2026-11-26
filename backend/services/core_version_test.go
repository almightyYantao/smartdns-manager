package services

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoreReleaseSelection(t *testing.T) {
	for _, item := range []struct {
		current, proposed string
		upgrade           bool
	}{
		{"Release48.4", "Release48.5", true},
		{"Release48.9", "Release48.10", true},
		{"Release48.5", "Release48.5", false},
		{"Release49", "Release48.5", false},
		{"unknown", "Release48.5", false},
	} {
		order, known := compareCoreTags(item.current, item.proposed)
		if (known && order < 0) != item.upgrade {
			t.Fatalf("%+v: order=%d known=%t", item, order, known)
		}
	}
	var release CoreRelease
	digest := strings.Repeat("a", 64)
	if err := json.Unmarshal([]byte(`{"tag_name":"Release48.5","assets":[{"name":"smartdns-x86_64","digest":"sha256:`+digest+`"},{"name":"smartdns-aarch64"}]}`), &release); err != nil {
		t.Fatal(err)
	}
	release.baseURL = coreProject + "download"
	url, got, ok := release.asset("x86_64")
	if !ok || got != digest || url != coreProject+"download/Release48.5/smartdns-x86_64" {
		t.Fatal(url, got, ok)
	}
	for _, arch := range []string{"aarch64", "arm", "../x86_64"} {
		if _, _, ok := release.asset(arch); ok {
			t.Fatalf("意外接受 %s", arch)
		}
	}
	release.Tag = "Release48.5;echo"
	if _, _, ok := release.asset("x86_64"); ok {
		t.Fatal("意外接受非法标签")
	}
}

func TestUpgradeInputAndScript(t *testing.T) {
	for _, name := range []string{"-option", "example.com;echo", "example.com\nexample.net", "a..com"} {
		if coreProbeName.MatchString(name) {
			t.Fatalf("意外接受 %q", name)
		}
	}
	if _, err := parseExpectedIPv4("::1"); err == nil {
		t.Fatal("意外接受 IPv6")
	}
	cmd := exec.Command("sh", "-n")
	cmd.Stdin = strings.NewReader(coreUpgradeScript)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestCoreMirrorAndDigest(t *testing.T) {
	source := CoreSource{"https://mirror.example.com/latest.json", "https://mirror.example.com/smartdns"}
	if err := source.validate(); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"http://mirror.example.com/latest.json", "https://user@mirror.example.com/", "https://mirror.example.com/?token=example"} {
		if err := (CoreSource{invalid, source.AssetBaseURL}).validate(); err == nil {
			t.Fatalf("意外接受 %s", invalid)
		}
	}
	const digest = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err := verifyCoreAsset([]byte("abc"), digest); err != nil {
		t.Fatal(err)
	}
	if err := verifyCoreAsset([]byte("<html>mirror page</html>"), digest); err == nil {
		t.Fatal("镜像错误页面通过了校验")
	}
}

func TestRunningCoreVersionUsesProcessDirectory(t *testing.T) {
	process := t.TempDir()
	cwd := filepath.Join(process, "cwd")
	if err := os.MkdirAll(filepath.Join(cwd, "lib"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "lib", "loader"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(process, "exe"), []byte("#!/bin/sh\ntest -f lib/loader || exit 126\necho 'smartdns (Release48.4)'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", coreRunningVersion+`running_core_version "$1"`, "sh", process)
	if out, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "smartdns (Release48.4)" {
		t.Fatalf("读取运行版本失败 %q: %v", out, err)
	}
}

func TestCoreDNSProbeRequiresAnswerOverlap(t *testing.T) {
	for _, tc := range []struct {
		status, records string
		want            bool
	}{
		{"NOERROR", "example.com. 60 IN A 192.0.2.9\nexample.com. 60 IN A 192.0.2.2", true},
		{"NOERROR", "example.com. 60 IN A 192.0.2.9", false},
		{"NOERROR", "", false},
		{"SERVFAIL", "example.com. 60 IN A 192.0.2.2", false},
	} {
		cmd := exec.Command("sh", "-c", coreDNSProbe+`
name=example.com.
baseline='192.0.2.1
192.0.2.2'
dig() {
 [ "$1" = @127.0.0.1 ] || return 1
 printf ';; status: %s,\n%s\n' "$probe_status" "$probe_records"
}
probe
`)
		cmd.Env = append(os.Environ(), "probe_status="+tc.status, "probe_records="+tc.records)
		if err := cmd.Run(); (err == nil) != tc.want {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func TestRunningCoreVersionFallsBackToInstallPrefix(t *testing.T) {
	process := t.TempDir()
	prefix := t.TempDir()
	if err := os.MkdirAll(filepath.Join(process, "cwd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(prefix, "sbin"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(prefix, "sbin", "smartdns")
	script := `#!/bin/sh
if [ "$PWD" != "$expected_prefix" ]; then
 echo 'bad ELF interpreter' >&2
 exit 126
fi
echo 'smartdns (Release48.4)'
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, filepath.Join(process, "exe")); err != nil {
		t.Fatal(err)
	}
	// 测试使用临时安装目录替代生产白名单路径。
	function := strings.Replace(coreRunningVersion, "/usr/sbin/smartdns", `"$expected_prefix/sbin/smartdns"`, 1)
	cmd := exec.Command("sh", "-c", function+`running_core_version "$1"`, "sh", process)
	cmd.Env = append(os.Environ(), "expected_prefix="+prefix)
	if out, err := cmd.CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "smartdns (Release48.4)" {
		t.Fatalf("安装目录回退失败 %q: %v", out, err)
	}
}
