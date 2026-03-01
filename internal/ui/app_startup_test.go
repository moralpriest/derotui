// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import "testing"

func TestApplyCLIStartupFlagsGenerateNew(t *testing.T) {
	m := NewModel()
	m.Opts.GenerateNew = true

	handled := m.ApplyCLIStartupFlags()
	if !handled {
		t.Fatalf("expected startup flow to be handled")
	}
	if m.page != PagePassword {
		t.Fatalf("expected PagePassword, got %v", m.page)
	}
	if !m.isCreating || m.isRestoringFromSeed || m.isRestoringFromKey {
		t.Fatalf("unexpected create/restore flags: create=%t restoreSeed=%t restoreKey=%t", m.isCreating, m.isRestoringFromSeed, m.isRestoringFromKey)
	}
}

func TestApplyCLIStartupFlagsRestoreSeedWithProvidedSeed(t *testing.T) {
	m := NewModel()
	m.Opts.RestoreSeed = true
	m.Opts.ElectrumSeed = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon"

	handled := m.ApplyCLIStartupFlags()
	if !handled {
		t.Fatalf("expected startup flow to be handled")
	}
	if m.page != PagePassword {
		t.Fatalf("expected PagePassword when seed is provided, got %v", m.page)
	}
	if m.isCreating || !m.isRestoringFromSeed || m.isRestoringFromKey {
		t.Fatalf("unexpected create/restore flags: create=%t restoreSeed=%t restoreKey=%t", m.isCreating, m.isRestoringFromSeed, m.isRestoringFromKey)
	}
}

func TestApplyCLIStartupFlagsRestoreSeedWithoutSeed(t *testing.T) {
	m := NewModel()
	m.Opts.RestoreSeed = true

	handled := m.ApplyCLIStartupFlags()
	if !handled {
		t.Fatalf("expected startup flow to be handled")
	}
	if m.page != PageSeed {
		t.Fatalf("expected PageSeed when seed is not provided, got %v", m.page)
	}
}
