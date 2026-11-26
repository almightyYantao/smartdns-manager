package services

import (
	"encoding/json"
	"os/exec"
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
