// Copyright 2017-2026 DERO Project. All rights reserved.

package installer

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadAndExtractDerod downloads the release asset and extracts derod to the target path.
func DownloadAndExtractDerod(plan Plan) error {
	if strings.TrimSpace(plan.AssetURL) == "" {
		return fmt.Errorf("missing asset download URL")
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, plan.AssetURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "derotui")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(plan.BinaryTarget), 0700); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(plan.BinaryTarget), "derotui-asset-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if strings.HasSuffix(strings.ToLower(plan.AssetName), ".zip") {
		return extractZipBinary(tmpPath, plan.BinaryTarget)
	}
	return extractTarGzBinary(tmpPath, plan.BinaryTarget)
}

func extractTarGzBinary(archivePath, targetPath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(header.Name)
		if base != "derod" && !strings.HasPrefix(base, "derod-") {
			continue
		}
		return writeExtractedBinary(targetPath, tarReader, 0755)
	}
	return fmt.Errorf("derod binary not found in archive")
}

func extractZipBinary(archivePath, targetPath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if base != "derod.exe" && base != "derod" && !strings.HasPrefix(base, "derod-") {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeExtractedBinary(targetPath, rc, 0755)
	}
	return fmt.Errorf("derod binary not found in archive")
}

func writeExtractedBinary(targetPath string, src io.Reader, mode os.FileMode) error {
	tmpTarget := targetPath + ".tmp"
	file, err := os.OpenFile(tmpTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, src); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpTarget, targetPath); err != nil {
		return err
	}
	return nil
}
