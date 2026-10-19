package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionComparison(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"2.0.15", "2.0.14", 1},
		{"2.1.0", "2.0.99", 1},
		{"2.0.15", "2.0.15", 0},
		{"1.8.4", "2.0.15", -1},
	}
	for _, test := range tests {
		if got := compareVersions(test.left, test.right); got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
	for _, invalid := range []string{"", "2", "2.0", "2.0.0-beta", "v2.a.0"} {
		if _, err := normalizeVersion(invalid); err == nil {
			t.Errorf("normalizeVersion(%q) unexpectedly succeeded", invalid)
		}
	}
}

func TestUpdaterIgnoresOlderRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(githubRelease{TagName: "v1.8.4", Name: "old"})
	}))
	defer server.Close()
	u := &githubUpdater{apiURL: server.URL, current: "2.0.15", client: server.Client(), allowAssetURL: func(*url.URL) bool { return true }}
	release, err := u.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.info.Available || release.info.LatestVersion != "1.8.4" {
		t.Fatalf("older release was not ignored: %+v", release.info)
	}
}

func TestUpdaterDownloadsAndVerifiesRelease(t *testing.T) {
	archive := []byte("signed update archive fixture")
	digest := sha256.Sum256(archive)
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "laosun-qiangnimei-2026-v2.0.16-macos-universal.zip"
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: "v2.0.16",
				Name:    "OTA test",
				Assets: []githubReleaseAsset{
					{Name: name, BrowserDownloadURL: server.URL + "/archive", Size: int64(len(archive))},
					{Name: name + ".sha256", BrowserDownloadURL: server.URL + "/checksum", Size: 128},
				},
			})
		case "/archive":
			_, _ = w.Write(archive)
		case "/checksum":
			_, _ = w.Write([]byte(hex.EncodeToString(digest[:]) + "  " + name + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	u := &githubUpdater{apiURL: server.URL + "/latest", current: "2.0.15", client: server.Client(), allowAssetURL: func(*url.URL) bool { return true }}
	release, err := u.latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !release.info.Available || release.info.LatestVersion != "2.0.16" {
		t.Fatalf("new release not detected: %+v", release.info)
	}
	dir := t.TempDir()
	path, err := u.downloadAndVerify(context.Background(), release, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(archive) || filepath.Base(path) != release.archiveName {
		t.Fatalf("downloaded archive mismatch: %q", got)
	}
}

func TestUpdaterRejectsChecksumMismatch(t *testing.T) {
	archive := []byte("tampered")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/archive" {
			_, _ = w.Write(archive)
			return
		}
		_, _ = w.Write([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa  update.zip\n"))
	}))
	defer server.Close()
	u := &githubUpdater{current: "2.0.15", client: server.Client(), allowAssetURL: func(*url.URL) bool { return true }}
	_, err := u.downloadAndVerify(context.Background(), updateRelease{archiveName: "update.zip", archiveURL: server.URL + "/archive", checksumURL: server.URL + "/checksum"}, t.TempDir())
	if err == nil {
		t.Fatal("checksum mismatch was accepted")
	}
}

func TestTrustedGitHubAssetURL(t *testing.T) {
	for _, raw := range []string{
		"https://github.com/Sunbelife/apple-store-helper-15/releases/download/v2.0.15/update.zip",
		"https://objects.githubusercontent.com/example",
		"https://release-assets.githubusercontent.com/example",
	} {
		value, _ := url.Parse(raw)
		if !isTrustedGitHubAssetURL(value) {
			t.Errorf("trusted URL rejected: %s", raw)
		}
	}
	for _, raw := range []string{"http://github.com/update.zip", "https://github.com.evil.example/update.zip", "https://user@github.com/update.zip"} {
		value, _ := url.Parse(raw)
		if isTrustedGitHubAssetURL(value) {
			t.Errorf("untrusted URL accepted: %s", raw)
		}
	}
}

func TestLiveGitHubUpdateCheck(t *testing.T) {
	if os.Getenv("QNM_UPDATE_LIVE") != "1" {
		t.Skip("set QNM_UPDATE_LIVE=1 to query GitHub Releases")
	}
	release, err := newGitHubUpdater().latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if release.info.LatestVersion == "" {
		t.Fatal("GitHub latest release returned no version")
	}
	t.Logf("current=%s latest=%s available=%v", release.info.CurrentVersion, release.info.LatestVersion, release.info.Available)
}
