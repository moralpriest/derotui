// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// diagPath is the file that records page transitions and daemon-page input.
// It is written unconditionally (no --debug flag required) so an unexpected
// navigation can be traced without interacting with the debug console.
func diagPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".derotui", "diag.log")
}

var diagMu sync.Mutex

// diagLog appends a timestamped line to the diagnostic log. Failures are
// ignored — diagnostics must never disrupt the TUI.
func diagLog(format string, args ...interface{}) {
	diagMu.Lock()
	defer diagMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(diagPath()), 0700); err != nil {
		return
	}
	f, err := os.OpenFile(diagPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
