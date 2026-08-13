// Copyright 2017-2026 DERO Project. All rights reserved.

package releases

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Asset describes a release asset.
type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

// Release describes a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Match is the selected release asset for derod.
type Match struct {
	TagName string
	Asset   Asset
}

// DiscoverOfficialDerod finds the best matching derod asset.
func DiscoverOfficialDerod(source string) (Match, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return Match{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "derotui")
	resp, err := client.Do(req)
	if err != nil {
		return Match{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Match{}, fmt.Errorf("release discovery failed with status %d", resp.StatusCode)
	}
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return Match{}, err
	}
	expectedNames := expectedAssetNames(runtime.GOOS, runtime.GOARCH)
	for _, release := range releases {
		for _, asset := range release.Assets {
			name := strings.ToLower(asset.Name)
			for _, expected := range expectedNames {
				if name == expected {
					return Match{TagName: release.TagName, Asset: asset}, nil
				}
			}
		}
		for _, asset := range release.Assets {
			name := strings.ToLower(asset.Name)
			if isCompatibleAsset(name, runtime.GOOS, runtime.GOARCH) {
				return Match{TagName: release.TagName, Asset: asset}, nil
			}
		}
	}
	return Match{}, fmt.Errorf("no dero release asset found for %s/%s", strings.ToLower(runtime.GOOS), strings.ToLower(runtime.GOARCH))
}

func expectedAssetNames(goos, goarch string) []string {
	goos = strings.ToLower(goos)
	goarch = strings.ToLower(goarch)
	switch goos {
	case "linux":
		return []string{"dero_linux_" + goarch + ".tar.gz"}
	case "windows":
		return []string{"dero_windows_" + goarch + ".zip"}
	case "darwin":
		return []string{"dero_darwin_universal.tar.gz"}
	case "freebsd":
		return []string{"dero_freebsd_" + goarch + ".tar.gz"}
	default:
		return nil
	}
}

func isCompatibleAsset(name, goos, goarch string) bool {
	name = strings.ToLower(name)
	goos = strings.ToLower(goos)
	goarch = strings.ToLower(goarch)
	if !strings.HasPrefix(name, "dero_") {
		return false
	}
	if goos == "darwin" {
		return strings.Contains(name, "darwin") && strings.HasSuffix(name, ".tar.gz")
	}
	if !strings.Contains(name, goos) || !strings.Contains(name, goarch) {
		return false
	}
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip")
}
