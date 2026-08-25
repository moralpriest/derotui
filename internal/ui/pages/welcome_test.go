// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// TestWelcomeFrameStaysAligned renders the welcome page with the densest daemon
// status summaries and asserts no content line pushes the frame's side border
// past its top/bottom border. Regression: leaving the /daemon page refreshed
// the welcome daemon status, which widened the summary line past the frame's
// inner width and visibly broke the outer border.
func TestWelcomeFrameStaysAligned(t *testing.T) {
	states := []DaemonStatusInfo{
		{ // bootstrapping from genesis with peer target
			IsOnline: true, IsHealthy: true, IsBootstrapping: true, SyncProgress: 0.05,
			Network: "Mainnet", Address: "localhost:10102", PeerHeight: 7_525_921,
		},
		{ // syncing with peer + pct on the widest network
			IsOnline: true, IsHealthy: true, IsSyncing: true, SyncProgress: 12.5,
			Network: "Simulator", Address: "127.0.0.1:20000", BlockHeight: 532, PeerHeight: 7_414_000,
		},
		{ // synced with peer height
			IsOnline: true, IsHealthy: true, IsSynced: true,
			Network: "Testnet", Address: "localhost:40402", BlockHeight: 7_414_000, PeerHeight: 7_414_000,
		},
		{ // stopped daemon
			Network: "Mainnet", Address: "Not configured",
		},
	}

	boxWidth := styles.Width
	for i, daemon := range states {
		w := NewWelcome()
		w.Version = "0.1.1"
		w.Daemons = []DaemonStatusInfo{daemon}
		lines := strings.Split(w.View(), "\n")
		for ln, line := range lines {
			if wl := lipgloss.Width(line); wl > boxWidth {
				t.Errorf("state %d line %d width %d > box %d: %q", i, ln, wl, boxWidth, stripANSI(line))
			}
			// Every row must be exactly the frame width: a shorter row means the
			// right border is missing/detached from the corner.
			if wl := lipgloss.Width(line); wl != boxWidth {
				t.Errorf("state %d line %d width %d != box width %d: %q", i, ln, wl, boxWidth, stripANSI(line))
			}
		}
	}
}
