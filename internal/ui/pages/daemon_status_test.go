// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/deroproject/dero-wallet-cli/internal/services/installer"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func planPreviewKeys(msg string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: msg}
}

// stripANSI removes lipgloss escape codes so footer key labels can be
// asserted as plain text.
func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// While an install plan is showing, Y must request apply and the normal page
// keys (start/stop/install...) must be inert.
func TestDaemonStatusInstallPlanBlocksPageKeys(t *testing.T) {
	d := NewDaemonStatus()
	d.SetInstallPlan(&installer.Plan{ServiceType: "System Service (Recommended)", BinaryTarget: "/home/u/.derotui/derod"})

	// A normal page key (x = stop) must not fire while the plan is up.
	next, _ := d.Update(planPreviewKeys("x"))
	if next.WantStop() || next.WantInstall() || next.WantStart() {
		t.Fatal("page keys must be blocked while an install plan is pending")
	}
	if next.InstallPlan == nil {
		t.Fatal("plan should still be pending after unrelated key")
	}

	// Y confirms the install.
	next, _ = next.Update(planPreviewKeys("y"))
	if !next.WantInstallApply() {
		t.Fatal("expected WantInstallApply after y")
	}
	if next.WantInstallDone() {
		t.Fatal("y must not also set done")
	}

	// N cancels.
	next.ResetActions()
	next, _ = next.Update(planPreviewKeys("n"))
	if !next.WantInstallDone() {
		t.Fatal("expected WantInstallDone after n")
	}
	if next.WantInstallApply() {
		t.Fatal("n must not set apply")
	}
}

// Esc cancels a pending install plan without leaving the page.
func TestDaemonStatusInstallPlanEscCancels(t *testing.T) {
	d := NewDaemonStatus()
	d.SetInstallPlan(&installer.Plan{})

	next, _ := d.Update(planPreviewKeys("esc"))
	if !next.WantInstallDone() {
		t.Fatal("expected WantInstallDone after esc")
	}
	if next.Cancelled() {
		t.Fatal("esc on an install plan must not navigate away")
	}
}

// Once a plan is applied or cancelled, normal page keys work again.
func TestDaemonStatusInstallPlanClearedRestoresKeys(t *testing.T) {
	d := NewDaemonStatus()
	d.SetInstallPlan(&installer.Plan{})
	d.ResetInstall()

	next, _ := d.Update(planPreviewKeys("x"))
	if !next.WantStop() {
		t.Fatal("expected WantStop after plan cleared")
	}
}

// The install plan renders its details and the confirmation prompt.
func TestDaemonStatusRendersInstallPlan(t *testing.T) {
	d := NewDaemonStatus()
	d.SetInstallPlan(&installer.Plan{
		ServiceType:  "System Service (Recommended)",
		ReleaseTag:   "v3.6.0",
		BinaryTarget: "/home/u/.derotui/derod",
		DataDir:      "/home/u/.derotui",
		RPCBind:      "127.0.0.1:10102",
		P2PBind:      "0.0.0.0:10101",
		GetWorkBind:  "0.0.0.0:10100",
		Network:      "mainnet",
		FallbackNote: "If system service install later fails, derotui can fall back to a user service.",
	})

	view := d.View()
	for _, want := range []string{"Install derod", "System Service (Recommended)", "/home/u/.derotui/derod", "127.0.0.1:10102", "[Y] Install"} {
		if !strings.Contains(view, want) {
			t.Errorf("install plan view missing %q", want)
		}
	}
}

// Install is hidden from the footer when a daemon is already running.
func TestDaemonStatusFooterHidesInstallWhenRunning(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Running: true, Managed: false, Source: "External Local"})

	view := stripANSI(d.View())
	if strings.Contains(view, "I Install") {
		t.Fatal("Install key must be hidden while a daemon is running")
	}
	if !strings.Contains(view, "X Stop") {
		t.Fatal("expected Stop in footer for a running daemon")
	}
}

// Install is offered when nothing is configured or running.
func TestDaemonStatusFooterShowsInstallWhenPlanned(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Running: false, Managed: false, Source: "Planned Local"})

	view := stripANSI(d.View())
	if !strings.Contains(view, "I Install") {
		t.Fatal("expected Install in footer for an unconfigured daemon")
	}
}
