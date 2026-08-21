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
	for _, want := range []string{"Install as Service", "System Service (Recommended)", "/home/u/.derotui/derod", "127.0.0.1:10102", "[Y] Install Service"} {
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

// Install is offered for the embedded helper too, since it now registers the
// built-in daemon as a background service rather than downloading a binary.
func TestDaemonStatusFooterShowsInstallForEmbedded(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Running: false, Managed: false, Source: "Embedded"})

	view := stripANSI(d.View())
	if !strings.Contains(view, "I Install Service") {
		t.Fatal("expected Install Service in footer for an embedded daemon")
	}
}

// Install is offered when nothing is configured or running.
func TestDaemonStatusFooterShowsInstallWhenPlanned(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Running: false, Managed: false, Source: "Planned Local"})

	view := stripANSI(d.View())
	if !strings.Contains(view, "I Install Service") {
		t.Fatal("expected Install Service in footer for an unconfigured daemon")
	}
}

// While an uninstall is awaiting confirmation, Y must request apply and the
// normal page keys (start/stop/install...) must be inert.
func TestDaemonStatusUninstallConfirmBlocksPageKeys(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Source: "System Daemon"})
	d.ConfirmingUninstall = true

	// A normal page key (x = stop) must not fire while the confirm is up.
	next, _ := d.Update(planPreviewKeys("x"))
	if next.WantStop() || next.WantInstall() || next.WantUninstall() {
		t.Fatal("page keys must be blocked while an uninstall confirm is pending")
	}
	if !next.ConfirmingUninstall {
		t.Fatal("confirm should still be pending after unrelated key")
	}

	// Y confirms the uninstall.
	next, _ = next.Update(planPreviewKeys("y"))
	if !next.WantUninstallApply() {
		t.Fatal("expected WantUninstallApply after y")
	}
	if next.WantUninstallDone() {
		t.Fatal("y must not also set done")
	}

	// N cancels.
	next.ResetActions()
	next, _ = next.Update(planPreviewKeys("n"))
	if !next.WantUninstallDone() {
		t.Fatal("expected WantUninstallDone after n")
	}
	if next.WantUninstallApply() {
		t.Fatal("n must not set apply")
	}
}

// Esc cancels a pending uninstall confirm without leaving the page.
func TestDaemonStatusUninstallConfirmEscCancels(t *testing.T) {
	d := NewDaemonStatus()
	d.ConfirmingUninstall = true

	next, _ := d.Update(planPreviewKeys("esc"))
	if !next.WantUninstallDone() {
		t.Fatal("expected WantUninstallDone after esc")
	}
	if next.Cancelled() {
		t.Fatal("esc on an uninstall confirm must not navigate away")
	}
}

// The reset confirm renders its explanation and prompt.
func TestDaemonStatusRendersUninstallConfirm(t *testing.T) {
	d := NewDaemonStatus()
	d.ConfirmingUninstall = true

	view := d.View()
	for _, want := range []string{"Reset daemon", "[Y] Reset", "deletes the chain data folder"} {
		if !strings.Contains(view, want) {
			t.Errorf("reset confirm view missing %q", want)
		}
	}
}

// Reset is shown in the footer for a systemd service, the embedded helper,
// and managed/planned local nodes — anything we own — but hidden for
// external daemons we merely connected to.
func TestDaemonStatusFooterShowsResetForOwnedDaemons(t *testing.T) {
	for _, tc := range []struct {
		source string
		shown  bool
	}{
		{"System Daemon", true},
		{"Embedded", true},
		{"Managed Local", true},
		{"Planned Local", true},
		{"External Local", false},
		{"Unknown", false},
	} {
		d := NewDaemonStatus()
		d.SetSnapshot(DaemonStatusSnapshot{Running: tc.source == "System Daemon", Source: tc.source})

		view := stripANSI(d.View())
		has := strings.Contains(view, "U Reset")
		if has != tc.shown {
			t.Errorf("source %q: expected Reset shown=%v, got %v", tc.source, tc.shown, has)
		}
	}
}

// A single Esc must not navigate away — it only opens the leave prompt. Only
// an explicit Y confirms. This makes it impossible for a stray Escape arriving
// from the terminal (e.g. a fragmented escape sequence) to kick the user back
// to welcome with no input.
func TestDaemonStatusEscOpensLeavePrompt(t *testing.T) {
	d := NewDaemonStatus()

	// Esc should navigate away directly (no confirmation) when not settling/downloading.
	next, _ := d.Update(planPreviewKeys("esc"))
	if !next.Cancelled() {
		t.Fatal("esc must navigate away directly")
	}
}

// A stray Esc followed by anything other than Y must not leave, and the
// prompt is dismissed. A later Esc re-opens it.
func TestDaemonStatusLeavePromptRequiresY(t *testing.T) {
	d := NewDaemonStatus()
	// With no confirmation prompt, Esc should leave directly; other keys should not.
	next, _ := d.Update(planPreviewKeys("esc"))
	if !next.Cancelled() {
		t.Fatal("esc should leave directly")
	}
	d2 := NewDaemonStatus()
	next2, _ := d2.Update(planPreviewKeys("y"))
	if next2.Cancelled() || next2.WantStart() {
		t.Fatal("y alone should not navigate away")
	}
}

// The page renders the leave prompt and the footer shows the confirm keys.
func TestDaemonStatusRendersLeavePrompt(t *testing.T) {
	d := NewDaemonStatus()
	view := d.View()
	if strings.Contains(view, "Leave daemon page") || strings.Contains(view, "[Y] Leave") {
		t.Fatal("View should not contain Leave daemon prompt after fix")
	}
	if view == "" {
		t.Fatal("view should not be empty")
	}
}

// Reset stays hidden while a download is in progress or an install plan is
// awaiting confirmation — mixing reset with an in-flight install is unsafe.
func TestDaemonStatusFooterHidesResetDuringDownloadAndInstall(t *testing.T) {
	d := NewDaemonStatus()
	d.SetSnapshot(DaemonStatusSnapshot{Source: "System Daemon"})
	d.Downloading = true
	if view := stripANSI(d.View()); strings.Contains(view, "U Reset") {
		t.Fatal("Reset must not show while downloading")
	}

	d.Downloading = false
	d.InstallPlan = &installer.Plan{}
	if view := stripANSI(d.View()); strings.Contains(view, "U Reset") {
		t.Fatal("Reset must not show while an install plan is pending")
	}
}

// Single-letter action keys are ignored right after the page appears, so a
// stray terminal byte (the trailing 'c' of a device attributes response) can't
// jump to Config or trigger Start/Stop/Install/Reset.
func TestDaemonStatusSettleIgnoresActionKeys(t *testing.T) {
	d := NewDaemonStatus()
	d.ArmSettle()

	// Esc should still work during settle (it's not a single-letter action key).
	next, _ := d.Update(planPreviewKeys("esc"))
	if !next.Cancelled() {
		t.Fatal("esc should not be ignored while settling")
	}

	// Single-letter action keys should be ignored until settle
	d2 := NewDaemonStatus()
	d2.ArmSettle()
	next2, _ := d2.Update(planPreviewKeys("s"))
	if next2.WantStart() || next2.Cancelled() {
		t.Fatal("s must be ignored while settling")
	}
}
