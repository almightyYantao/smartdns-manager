package services

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"
)

type VersionService struct {
	currentVersion string
	repoOwner      string
	repoName       string
}

type GitHubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
		URL string `json:"url"`
	} `json:"commit"`
}

type UpdateNotification struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	Description    string `json:"description,omitempty"`
}

type Version struct {
	Major int
	Minor int
	Patch int
	Raw   string
}

func NewVersionService(currentVersion, repoOwner, repoName string) *VersionService {
	return &VersionService{
		currentVersion: currentVersion,
		repoOwner:      repoOwner,
		repoName:       repoName,
	}
}

// CheckForUpdates 检查是否有新版本
func (v *VersionService) CheckForUpdates() (*UpdateNotification, error) {
	log.Printf("🔍 检查版本更新，当前版本: %s", v.currentVersion)

	// 获取所有 docker- 开头的 tags
	dockerTags, err := v.getDockerTags()
	if err != nil {
		return nil, fmt.Errorf("获取 Docker tags 失败: %w", err)
	}

	if len(dockerTags) == 0 {
		return &UpdateNotification{
			HasUpdate:      false,
			CurrentVersion: v.currentVersion,
			LatestVersion:  v.currentVersion,
			Description:    "未找到可用的 Docker 版本标签",
		}, nil
	}

	// 找到最新版本
	latestVersion := v.findLatestVersion(dockerTags)

	notification := &UpdateNotification{
		CurrentVersion: v.currentVersion,
		LatestVersion:  latestVersion,
	}

	// 比较版本
	if v.isNewerVersion(v.currentVersion, latestVersion) {
		notification.HasUpdate = true
		notification.Description = fmt.Sprintf("发现新版本 %s，建议及时更新", latestVersion)
		log.Printf("🆕 发现新版本: %s -> %s", v.currentVersion, latestVersion)
	} else {
		notification.HasUpdate = false
		notification.Description = "当前版本已是最新版本"
		log.Printf("✅ 当前版本已是最新: %s", v.currentVersion)
	}

	return notification, nil
}

// getDockerTags 获取所有 docker- 开头的 tags
func (v *VersionService) getDockerTags() ([]string, error) {
	// GitHub API URL for tags
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=100", v.repoOwner, v.repoName)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回错误状态码: %d", resp.StatusCode)
	}

	var tags []GitHubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, fmt.Errorf("解析 GitHub API 响应失败: %w", err)
	}

	// 过滤出 docker- 开头的 tags
	var dockerTags []string
	dockerTagPattern := regexp.MustCompile(`^docker-v\d+\.\d+\.\d+$`)

	for _, tag := range tags {
		if dockerTagPattern.MatchString(tag.Name) {
			dockerTags = append(dockerTags, tag.Name)
			log.Printf("📦 找到 Docker tag: %s", tag.Name)
		}
	}

	log.Printf("📋 共找到 %d 个 Docker 版本标签", len(dockerTags))
	return dockerTags, nil
}

// findLatestVersion 从 tags 列表中找到最新版本
func (v *VersionService) findLatestVersion(tags []string) string {
	if len(tags) == 0 {
		return v.currentVersion
	}

	// 解析所有版本
	versions := make([]Version, 0, len(tags))
	for _, tag := range tags {
		if version := v.parseVersion(tag); version != nil {
			versions = append(versions, *version)
		}
	}

	if len(versions) == 0 {
		return v.currentVersion
	}

	// 排序找到最新版本
	sort.Slice(versions, func(i, j int) bool {
		a, b := versions[i], versions[j]
		if a.Major != b.Major {
			return a.Major > b.Major
		}
		if a.Minor != b.Minor {
			return a.Minor > b.Minor
		}
		return a.Patch > b.Patch
	})

	return versions[0].Raw
}

// parseVersion 解析版本字符串
func (v *VersionService) parseVersion(tag string) *Version {
	// 匹配 docker-v1.2.3 格式
	re := regexp.MustCompile(`^docker-v(\d+)\.(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(tag)

	if len(matches) != 4 {
		return nil
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
		Raw:   tag,
	}
}

// isNewerVersion 判断 latest 是否比 current 新
func (v *VersionService) isNewerVersion(current, latest string) bool {
	currentVer := v.parseVersion(current)
	latestVer := v.parseVersion(latest)

	if currentVer == nil || latestVer == nil {
		return false
	}

	// 比较版本号
	if latestVer.Major > currentVer.Major {
		return true
	}
	if latestVer.Major == currentVer.Major && latestVer.Minor > currentVer.Minor {
		return true
	}
	if latestVer.Major == currentVer.Major && latestVer.Minor == currentVer.Minor && latestVer.Patch > currentVer.Patch {
		return true
	}

	return false
}

// GetCurrentVersion 获取当前版本
func (v *VersionService) GetCurrentVersion() string {
	return v.currentVersion
}

// SetCurrentVersion 设置当前版本（更新后调用）
func (v *VersionService) SetCurrentVersion(version string) {
	v.currentVersion = version
	log.Printf("📝 更新当前版本: %s", version)
}

// GetVersionHistory 获取版本历史
func (v *VersionService) GetVersionHistory(limit int) ([]string, error) {
	dockerTags, err := v.getDockerTags()
	if err != nil {
		return nil, err
	}

	// 解析并排序
	versions := make([]Version, 0, len(dockerTags))
	for _, tag := range dockerTags {
		if version := v.parseVersion(tag); version != nil {
			versions = append(versions, *version)
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		a, b := versions[i], versions[j]
		if a.Major != b.Major {
			return a.Major > b.Major
		}
		if a.Minor != b.Minor {
			return a.Minor > b.Minor
		}
		return a.Patch > b.Patch
	})

	// 限制返回数量
	if limit > 0 && limit < len(versions) {
		versions = versions[:limit]
	}

	result := make([]string, len(versions))
	for i, v := range versions {
		result[i] = v.Raw
	}

	return result, nil
}
