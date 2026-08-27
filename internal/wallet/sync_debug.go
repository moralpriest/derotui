// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

// Temporary diagnostics for the app-side sync status. Remove after debugging.
import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var syncDbgAppMu sync.Mutex

func syncDbgAppf(format string, args ...interface{}) {
	syncDbgAppMu.Lock()
	defer syncDbgAppMu.Unlock()
	home, _ := os.UserHomeDir()
	if home == "" {
		return
	}
	_ = os.MkdirAll(filepath.Join(home, ".derotui"), 0700)
	f, err := os.OpenFile(filepath.Join(home, ".derotui", "walletapi-sync.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
