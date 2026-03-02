// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import "testing"

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
