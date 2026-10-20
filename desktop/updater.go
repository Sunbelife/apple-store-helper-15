package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	maxReleaseResponse = 2 << 20
	maxChecksumSize    = 8 << 10
	maxUpdateSize      = 250 << 20
)

type AppInfo struct {
	Platform   string `json:"platform"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
}

type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	Name           string `json:"name"`
	Notes          string `json:"notes"`
	PublishedAt    string `json:"publishedAt"`
	ReleaseURL     string `json:"releaseURL"`
	DownloadSize   int64  `json:"downloadSize"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt string               `json:"published_at"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type updateRelease struct {
	info        UpdateInfo
	archiveName string
	archiveURL  string
	checksumURL string
}

type githubUpdater struct {
	apiURL        string
	assetBaseURL  string
	current       string
	client        *http.Client
	allowAssetURL func(*url.URL) bool
	mu            sync.Mutex
	installing    bool
}

func newGitHubUpdater() *githubUpdater {
	return &githubUpdater{
		apiURL:        updateAPIURL,
		assetBaseURL:  updateDownloadBaseURL,
		current:       AppVersion,
		client:        &http.Client{Timeout: 10 * time.Minute},
		allowAssetURL: isTrustedUpdateAssetURL,
	}
}

func (a *Desktop) GetAppInfo() AppInfo {
	return AppInfo{Version: AppVersion, Repository: updateRepository, Platform: runtime.GOOS}
}

func (a *Desktop) OpenDownloads() {
	wruntime.BrowserOpenURL(a.ctx, "https://sunbelife.com/projects/qiangnimei/")
}

func (a *Desktop) CheckForUpdates() (UpdateInfo, error) {
	if runtime.GOOS != "darwin" {
		return UpdateInfo{CurrentVersion: AppVersion, LatestVersion: AppVersion}, nil
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := a.updater.latest(ctx)
	if err != nil {
		return UpdateInfo{}, err
	}
	return release.info, nil
}

func (a *Desktop) InstallUpdate() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("请到官网下载新版 Windows 安装包")
	}
	u := a.updater
	u.mu.Lock()
	if u.installing {
		u.mu.Unlock()
		return fmt.Errorf("更新正在下载，请稍候")
	}
	u.installing = true
	u.mu.Unlock()
	defer func() {
		u.mu.Lock()
		u.installing = false
		u.mu.Unlock()
	}()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	release, err := u.latest(ctx)
	if err != nil {
		return err
	}
	if !release.info.Available {
		return fmt.Errorf("当前已是最新版本")
	}
	root, err := os.MkdirTemp("", "qiangnimei-update-")
	if err != nil {
		return fmt.Errorf("创建更新目录失败：%w", err)
	}
	keepFiles := false
	defer func() {
		if !keepFiles {
			_ = os.RemoveAll(root)
		}
	}()
	archive, err := u.downloadAndVerify(ctx, release, root)
	if err != nil {
		return err
	}
	if err = prepareAndLaunchUpdate(root, archive, release.info.LatestVersion); err != nil {
		return err
	}
	keepFiles = true
	a.Pause()
	wruntime.Quit(a.ctx)
	return nil
}

func (u *githubUpdater) latest(ctx context.Context) (updateRelease, error) {
	var result updateRelease
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.apiURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "Laosun-Qiangnimei-"+u.current)
	resp, err := u.client.Do(req)
	if err != nil {
		return result, fmt.Errorf("检查 GitHub 更新失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("检查 GitHub 更新失败：HTTP %d", resp.StatusCode)
	}
	body, err := readLimited(resp.Body, maxReleaseResponse)
	if err != nil {
		return result, fmt.Errorf("读取 GitHub 更新信息失败：%w", err)
	}
	var release githubRelease
	if err = json.Unmarshal(body, &release); err != nil {
		return result, fmt.Errorf("解析 GitHub 更新信息失败：%w", err)
	}
	latest, err := normalizeVersion(release.TagName)
	if err != nil || release.Draft || release.Prerelease {
		return result, fmt.Errorf("GitHub 最新 Release 版本格式不可用")
	}
	current, err := normalizeVersion(u.current)
	if err != nil {
		return result, fmt.Errorf("当前应用版本格式不可用：%w", err)
	}
	result.info = UpdateInfo{
		CurrentVersion: current,
		LatestVersion:  latest,
		Available:      compareVersions(latest, current) > 0,
		Name:           strings.TrimSpace(release.Name),
		Notes:          strings.TrimSpace(release.Body),
		PublishedAt:    release.PublishedAt,
		ReleaseURL:     release.HTMLURL,
	}
	if result.info.Name == "" {
		result.info.Name = "v" + latest
	}
	if !result.info.Available {
		return result, nil
	}
	expected := fmt.Sprintf("laosun-qiangnimei-2026-v%s-macos-universal.zip", latest)
	hasArchive, hasChecksum := false, false
	for _, asset := range release.Assets {
		switch asset.Name {
		case expected:
			result.archiveName = asset.Name
			result.info.DownloadSize = asset.Size
			hasArchive = true
		case expected + ".sha256":
			hasChecksum = true
		}
	}
	if !hasArchive || !hasChecksum {
		return updateRelease{}, fmt.Errorf("GitHub Release 缺少 macOS 通用更新包或 SHA-256 校验文件")
	}
	if result.info.DownloadSize <= 0 || result.info.DownloadSize > maxUpdateSize {
		return updateRelease{}, fmt.Errorf("GitHub 更新包大小异常")
	}
	base := strings.TrimRight(u.assetBaseURL, "/")
	if base == "" {
		return updateRelease{}, fmt.Errorf("更新下载地址未配置")
	}
	result.archiveURL = base + "/v" + latest + "/" + expected
	result.checksumURL = result.archiveURL + ".sha256"
	for _, raw := range []string{result.archiveURL, result.checksumURL} {
		parsed, parseErr := url.Parse(raw)
		if parseErr != nil || !u.allowAssetURL(parsed) {
			return updateRelease{}, fmt.Errorf("GitHub 更新下载地址不可信")
		}
	}
	return result, nil
}

func (u *githubUpdater) downloadAndVerify(ctx context.Context, release updateRelease, root string) (string, error) {
	checksumBody, err := u.download(ctx, release.checksumURL, maxChecksumSize)
	if err != nil {
		return "", fmt.Errorf("下载更新校验文件失败：%w", err)
	}
	expected, err := parseChecksum(string(checksumBody), release.archiveName)
	if err != nil {
		return "", err
	}
	archiveBody, err := u.download(ctx, release.archiveURL, maxUpdateSize)
	if err != nil {
		return "", fmt.Errorf("下载更新包失败：%w", err)
	}
	actual := sha256.Sum256(archiveBody)
	if !strings.EqualFold(expected, hex.EncodeToString(actual[:])) {
		return "", fmt.Errorf("更新包 SHA-256 校验失败，已停止安装")
	}
	archive := filepath.Join(root, release.archiveName)
	if err = os.WriteFile(archive, archiveBody, 0600); err != nil {
		return "", fmt.Errorf("保存更新包失败：%w", err)
	}
	return archive, nil
}

func (u *githubUpdater) download(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "Laosun-Qiangnimei-"+u.current)
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.Request == nil || resp.Request.URL == nil || !u.allowAssetURL(resp.Request.URL) {
		return nil, fmt.Errorf("下载重定向地址不可信")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > maxBytes {
		return nil, fmt.Errorf("下载内容超过允许大小")
	}
	return readLimited(resp.Body, maxBytes)
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("响应内容超过允许大小")
	}
	return body, nil
}

func parseChecksum(value, filename string) (string, error) {
	fields := strings.Fields(value)
	if len(fields) < 1 || len(fields[0]) != sha256.Size*2 {
		return "", fmt.Errorf("更新包 SHA-256 校验文件格式错误")
	}
	if _, err := hex.DecodeString(fields[0]); err != nil {
		return "", fmt.Errorf("更新包 SHA-256 校验文件格式错误")
	}
	if len(fields) >= 2 && strings.TrimPrefix(fields[1], "*") != filename {
		return "", fmt.Errorf("SHA-256 校验文件与更新包名称不匹配")
	}
	return strings.ToLower(fields[0]), nil
}

func normalizeVersion(value string) (string, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "v"))
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("版本号必须为 major.minor.patch")
	}
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("版本号不能为空")
		}
		if _, err := strconv.ParseUint(part, 10, 32); err != nil {
			return "", fmt.Errorf("版本号必须为数字")
		}
	}
	return strings.Join(parts, "."), nil
}

func compareVersions(left, right string) int {
	l, _ := normalizeVersion(left)
	r, _ := normalizeVersion(right)
	lp, rp := strings.Split(l, "."), strings.Split(r, ".")
	for i := range lp {
		lv, _ := strconv.ParseUint(lp[i], 10, 32)
		rv, _ := strconv.ParseUint(rp[i], 10, 32)
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	return 0
}

func isTrustedUpdateAssetURL(value *url.URL) bool {
	if value == nil || value.Scheme != "https" || value.User != nil {
		return false
	}
	host := strings.ToLower(value.Hostname())
	return host == "sunbelife.com" || host == "ibeta-vue.oss-cn-hangzhou.aliyuncs.com"
}
