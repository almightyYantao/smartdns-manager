package config

import (
	"testing"
	"time"
)

func TestStaticAddressConfig(t *testing.T) {
	t.Setenv("NODE_ID", "1")
	t.Setenv("STATIC_ADDRESS_TAGGING", "false")
	t.Setenv("STATIC_ADDRESS_TAG_FROM", "")
	cfg, err := Load()
	if err != nil || cfg.StaticAddressTagging {
		t.Fatalf("默认关闭标记: %v", err)
	}
	t.Setenv("STATIC_ADDRESS_TAGGING", "true")
	t.Setenv("STATIC_ADDRESS_TAG_FROM", "2026-10-26T10:00:00+08:00")
	cfg, err = Load()
	if err != nil || !cfg.StaticAddressTagging || cfg.StaticAddressTagFrom.Format(time.RFC3339) != "2026-10-26T10:00:00+08:00" {
		t.Fatalf("显式配置未生效: %v", err)
	}
	t.Setenv("STATIC_ADDRESS_TAG_FROM", "invalid")
	if _, err := Load(); err == nil {
		t.Fatal("接受了无效启用时间")
	}
}
