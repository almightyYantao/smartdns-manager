package services

import (
	"archive/zip"
	"bytes"
	"context"
	"debug/elf"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const MaxCoreBundle = 18 << 20

func coreBundlePath() string {
	return filepath.Join(filepath.Dir(coreSourcePath()), "smartdns-core-bundle.zip")
}

func readBundleEntry(bundle *zip.Reader, name string, limit int64) ([]byte, error) {
	var entry *zip.File
	for _, file := range bundle.File {
		if file.Name != name {
			continue
		}
		if entry != nil || !file.Mode().IsRegular() || file.UncompressedSize64 > uint64(limit) {
			return nil, fmt.Errorf("离线包条目无效：%s", name)
		}
		entry = file
	}
	if entry == nil {
		return nil, fmt.Errorf("离线包缺少 %s", name)
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err == nil && int64(len(data)) > limit {
		return nil, fmt.Errorf("离线包条目过大：%s", name)
	}
	return data, err
}

func bundleRelease(bundle *zip.Reader) (CoreRelease, error) {
	var release CoreRelease
	data, err := readBundleEntry(bundle, "latest.json", 64<<10)
	if err != nil {
		return release, err
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return release, errors.New("离线包的 latest.json 格式无效")
	}
	if !coreTag.MatchString(release.Tag) || release.Draft || release.Prerelease {
		return release, errors.New("离线包必须包含稳定发行版")
	}
	release.offline = true
	return release, nil
}

func ImportCoreBundle(data []byte) (CoreRelease, error) {
	return importCoreBundle(coreBundlePath(), data)
}

func importCoreBundle(path string, data []byte) (CoreRelease, error) {
	if len(data) > MaxCoreBundle {
		return CoreRelease{}, errors.New("离线包超过 18 MiB")
	}
	bundle, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(bundle.File) > 3 {
		return CoreRelease{}, errors.New("请选择符合格式的 SmartDNS ZIP 离线包")
	}
	release, err := bundleRelease(bundle)
	if err != nil {
		return CoreRelease{}, err
	}
	verified := 0
	for arch, machine := range map[string]elf.Machine{"x86_64": elf.EM_X86_64, "aarch64": elf.EM_AARCH64} {
		name, digest, ok := release.asset(arch)
		if !ok {
			continue
		}
		asset, err := readBundleEntry(bundle, name, maxCoreAsset)
		if err != nil {
			return CoreRelease{}, err
		}
		if err := verifyCoreAsset(asset, digest); err != nil {
			return CoreRelease{}, err
		}
		binary, err := elf.NewFile(bytes.NewReader(asset))
		if err != nil || binary.Machine != machine || binary.Class != elf.ELFCLASS64 || (binary.Type != elf.ET_EXEC && binary.Type != elf.ET_DYN) {
			return CoreRelease{}, fmt.Errorf("升级包不是匹配架构的 ELF 可执行文件：%s", arch)
		}
		verified++
	}
	if verified == 0 || verified != len(release.Assets) || verified+1 != len(bundle.File) {
		return CoreRelease{}, errors.New("离线包必须只包含元数据及与其匹配的升级包")
	}
	return release, writeCoreFile(path, data)
}

func LoadCoreBundle() (CoreRelease, error) {
	bundle, err := zip.OpenReader(coreBundlePath())
	if err != nil {
		return CoreRelease{}, err
	}
	defer bundle.Close()
	return bundleRelease(&bundle.Reader)
}

func RemoveCoreBundle() error {
	err := os.Remove(coreBundlePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (r CoreRelease) readAsset(ctx context.Context, address string) ([]byte, error) {
	if !r.offline {
		return readCoreSource(ctx, address, maxCoreAsset, coreAssetTimeout)
	}
	bundle, err := zip.OpenReader(coreBundlePath())
	if err != nil {
		return nil, err
	}
	defer bundle.Close()
	return readBundleEntry(&bundle.Reader, address, maxCoreAsset)
}
