// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// TestDashboardFrameFitsWidth renders the dashboard (as shown after navigating
// back from the /daemon page) and asserts the outer frame never exceeds the
// design width. Regression: the framed dashboard came out 82 columns wide
// (dashboard content padded to styles.Width + frame borders), so in an
// 80-column terminal the right border was clipped and the frame looked
// misaligned next to every other page's 80-column box.
func TestDashboardFrameFitsWidth(t *testing.T) {
	states := []struct {
		name  string
		setup func(d *pages.DashboardModel)
	}{
		{"bootstrapping offline", func(d *pages.DashboardModel) {
			d.SetWalletInfo("main.db", "Mainnet", true, false, true, "127.0.0.1:10102", 0, 7_525_921)
			d.SetDaemonBootstrapping(true)
			d.SetFlashMessage("Wallet opened offline. Use /connect to retry. - dial tcp 127.0.0.1:10102: connect: connection refused", false)
		}},
		{"unregistered", func(d *pages.DashboardModel) {
			d.SetWalletInfo("main.db", "Mainnet", true, true, false, "127.0.0.1:10102", 7_525_921, 7_525_921)
			d.SetFlashMessage("Wallet not registered. Press [G] to register.", false)
		}},
		{"recent transactions", func(d *pages.DashboardModel) {
			d.SetWalletInfo("main.db", "Mainnet", true, true, true, "127.0.0.1:10102", 7_525_921, 7_525_921)
			d.SetRecentTxs([]pages.Transaction{
				{TxID: "abc", Amount: 1_000_000_000, Timestamp: "2026-08-25", Coinbase: true},
				{TxID: "def", Amount: -500_000_000, Timestamp: "2026-08-24"},
				{TxID: "ghi", Amount: 2_000_000_000, Timestamp: "2026-08-23"},
			})
		}},
		{"synced", func(d *pages.DashboardModel) {
			d.SetWalletInfo("main.db", "Mainnet", true, true, true, "127.0.0.1:10102", 7_525_921, 7_525_921)
		}},
	}

	boxWidth := styles.Width
	for _, st := range states {
		t.Run(st.name, func(t *testing.T) {
			m := NewModel()
			m.page = PageMain
			d := &m.dashboard
			d.SetBalance(1_234_567_890, 0)
			d.SetAddress("dero1qytestaddress")
			st.setup(d)

			lines := strings.Split(m.renderDashboard(), "\n")
			maxW := 0
			for _, line := range lines {
				if w := lipgloss.Width(line); w > maxW {
					maxW = w
				}
			}
			if maxW > boxWidth {
				t.Errorf("dashboard frame width %d exceeds %d", maxW, boxWidth)
			}
			// All frame rows must be the same width (top, sides, bottom).
			for i, line := range lines {
				if w := lipgloss.Width(line); w != maxW {
					t.Errorf("line %d width %d != frame width %d", i, w, maxW)
				}
			}
		})
	}
}
