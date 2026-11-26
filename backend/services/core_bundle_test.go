package services

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestCoreBundleImport(t *testing.T) {
	// 最小 ELF64 文件头，测试架构和摘要校验，无需执行二进制。
	asset := make([]byte, 64)
	copy(asset, "\x7fELF\x02\x01\x01")
	binary.LittleEndian.PutUint16(asset[16:], 2)
	binary.LittleEndian.PutUint16(asset[18:], 62)
	binary.LittleEndian.PutUint32(asset[20:], 1)
	binary.LittleEndian.PutUint16(asset[52:], 64)
	metadata := fmt.Sprintf(`{"tag_name":"Release48.5","assets":[{"name":"smartdns-x86_64","digest":"sha256:%x"}]}`, sha256.Sum256(asset))
	makeBundle := func(payload []byte) []byte {
		t.Helper()
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		for _, entry := range []struct {
			name string
			data []byte
		}{
			{"latest.json", []byte(metadata)}, {"Release48.5/smartdns-x86_64", payload},
		} {
			w, err := writer.Create(entry.name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	path := filepath.Join(t.TempDir(), "bundle.zip")
	valid := makeBundle(asset)
	release, err := importCoreBundle(path, valid)
	if err != nil || !release.offline || release.Tag != "Release48.5" {
		t.Fatalf("%+v %v", release, err)
	}
	if _, err := importCoreBundle(path, makeBundle([]byte("changed"))); err == nil {
		t.Fatal("错误摘要被接受")
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, valid) {
		t.Fatalf("导入失败后原包发生变化: %v", err)
	}
	if fixture := os.Getenv("SMARTDNS_TEST_RELEASE_ZIP"); fixture != "" {
		data, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := importCoreBundle(path, data); err != nil {
			t.Fatal(err)
		}
	}
}
