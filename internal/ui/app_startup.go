// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"github.com/deroproject/dero-wallet-cli/internal/ui/pages"
)

// ApplyCLIStartupFlags applies one-shot CLI startup intent to the initial UI state.
// It returns true when a startup flow was selected and should suppress auto-open-last-wallet.
func (m *Model) ApplyCLIStartupFlags() bool {
	if m.Opts.GenerateNew {
		m.isCreating = true
		m.isRestoringFromSeed = false
		m.isRestoringFromKey = false
		m.page = PagePassword
		m.password = pages.NewPassword(pages.PasswordModeCreate)
		m.password.SetVersion(Version)
		return true
	}

	if m.Opts.RestoreSeed {
		m.isCreating = false
		m.isRestoringFromSeed = true
		m.isRestoringFromKey = false
		if m.Opts.ElectrumSeed != "" {
			m.page = PagePassword
			m.password = pages.NewPassword(pages.PasswordModeCreate)
			m.password.SetVersion(Version)
		} else {
			m.seed = pages.NewSeed(pages.SeedModeInput, "")
			m.page = PageSeed
		}
		return true
	}

	return false
}

func (m *Model) SetStartupFlowSet(v bool) {
	m.startupFlowSet = v
}
