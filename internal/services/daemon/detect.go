// Copyright 2017-2026 DERO Project. All rights reserved.

package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateBinaryPath validates a derod binary path.
func ValidateBinaryPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("daemon binary path is not configured")
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("daemon binary path points to a directory")
	}
	if info.Mode()&0111 == 0 {
		return "", fmt.Errorf("daemon binary is not executable")
	}
	return abs, nil
}
