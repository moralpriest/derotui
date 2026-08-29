// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLogoEscBack(t *testing.T) {
	m := NewLogo()
	m, _ = m.Update(tea.KeyPressMsg{Text: "esc"})
	if !m.Cancelled() {
		t.Fatal("esc should cancel")
	}
	v := NewLogo().View()
	if !containsStr(v, "Back") {
		t.Fatalf("expected back hint, got %q", v)
	}
}
