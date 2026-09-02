// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"charm.land/lipgloss/v2"
	"strings"
	"testing"

	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

func TestTokenSendViewHasOuterFrame(t *testing.T) {
	s := NewTokenSend()
	s.SetToken("1111111111111111111111111111111111111111111111111111111111111111", "TKN", 5, 1_000_000, 1_000_000)

	view := s.View()
	lines := strings.Split(view, "\n")
	if len(lines) < 3 {
		t.Fatalf("expected framed token-send view, got %q", view)
	}
	plainTop := stripANSI(lines[0])
	plainBottom := stripANSI(lines[len(lines)-1])
	if !strings.HasPrefix(plainTop, "╭") || !strings.HasSuffix(plainTop, "╮") {
		t.Fatalf("missing top frame border: %q", plainTop)
	}
	if !strings.HasPrefix(plainBottom, "╰") || !strings.HasSuffix(plainBottom, "╯") {
		t.Fatalf("missing bottom frame border: %q", plainBottom)
	}
	for lineNumber, line := range lines {
		if width := lipgloss.Width(line); width != styles.Width {
			t.Errorf("line %d width %d, want %d", lineNumber, width, styles.Width)
		}
	}
}

func TestSendValidateAcceptsUsername(t *testing.T) {
	s := NewSend()
	s.SetBalance(1_000_000)
	s.addressInput.SetValue("alice")
	s.amountInput.SetValue("1")

	if got := s.validate(); got != "" {
		t.Fatalf("expected username to be valid, got error: %q", got)
	}
}

func TestSendValidateRejectsAtPrefixedUsername(t *testing.T) {
	s := NewSend()
	s.SetBalance(1_000_000)
	s.addressInput.SetValue("@alice")
	s.amountInput.SetValue("1")

	if got := s.validate(); got != "Invalid DERO address or username" {
		t.Fatalf("expected invalid recipient error, got: %q", got)
	}
}
