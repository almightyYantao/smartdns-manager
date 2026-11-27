package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"smartdns-manager/config"
)

const maxCoreAsset = 16 << 20
const coreAssetTimeout = 40 * time.Second

type CoreSource struct {
	MetadataURL  string `json:"metadata_url"`
	AssetBaseURL string `json:"asset_base_url"`
}

func defaultCoreSource() CoreSource {
	return CoreSource{"https://api.github.com/repos/pymumu/smartdns/releases/latest", coreProject + "download"}
}

func coreSourcePath() string {
	return filepath.Join(filepath.Dir(config.GetConfig().DBPath), "smartdns-core-source.json")
}

func LoadCoreSource() (CoreSource, error) {
	data, err := os.ReadFile(coreSourcePath())
	if errors.Is(err, os.ErrNotExist) {
		return defaultCoreSource(), nil
	}
	if err != nil {
		return CoreSource{}, err
	}
	var source CoreSource
	if err := json.Unmarshal(data, &source); err != nil {
		return source, err
	}
	return source, source.validate()
}

func (s CoreSource) validate() error {
	for _, address := range []string{s.MetadataURL, s.AssetBaseURL} {
		parsed, err := url.Parse(address)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" || parsed.ForceQuery {
			return errors.New("发布源必须是 HTTPS 地址，且不能包含凭据、查询参数或片段")
		}
	}
	return nil
}

func SaveCoreSource(source CoreSource) error {
	if err := source.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return writeCoreFile(coreSourcePath(), data)
}

func writeCoreFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".smartdns-core-source-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readCoreSource(ctx context.Context, address string, limit int64, timeout time.Duration) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(next *http.Request, previous []*http.Request) error {
		if len(previous) >= 5 || next.URL.Scheme != "https" || next.URL.User != nil {
			return errors.New("发布源重定向无效")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("发布源返回 HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("发布源响应超过大小限制")
	}
	return data, nil
}

func (s CoreSource) Latest(ctx context.Context) (CoreRelease, error) {
	var release CoreRelease
	if err := s.validate(); err != nil {
		return release, err
	}
	data, err := readCoreSource(ctx, s.MetadataURL, 2<<20, 8*time.Second)
	if err != nil {
		return release, err
	}
	if err := json.Unmarshal(data, &release); err != nil {
		return release, err
	}
	if !coreTag.MatchString(release.Tag) || release.Draft || release.Prerelease {
		return release, errors.New("发布元数据必须指向稳定发行版")
	}
	release.baseURL = strings.TrimRight(s.AssetBaseURL, "/")
	return release, nil
}

func verifyCoreAsset(data []byte, digest string) error {
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != digest {
		return errors.New("升级包 SHA-256 与发行元数据不符")
	}
	return nil
}

func (r CoreRelease) CheckAvailableAsset(ctx context.Context) error {
	for _, arch := range []string{"x86_64", "aarch64"} {
		address, digest, ok := r.asset(arch)
		if !ok {
			continue
		}
		data, err := readCoreSource(ctx, address, maxCoreAsset, 40*time.Second)
		if err == nil {
			err = verifyCoreAsset(data, digest)
		}
		return err
	}
	return errors.New("发行元数据没有可校验的升级包")
}
