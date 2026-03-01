// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/deroproject/dero-wallet-cli/internal/ui/styles"
)

// setWindowTitleCmd returns a command to set the window title based on current state.
func (m Model) setWindowTitleCmd() tea.Cmd {
	// Window title setting is handled differently in Bubble Tea v2
	// Returning nil as this is now managed at the program level
	return nil
}

// generateWindowTitle generates an appropriate window title based on current page and wallet state.
func (m Model) generateWindowTitle() string {
	base := "DERO"

	// Get wallet filename (without path) if wallet is open
	var walletName string
	if m.wallet != nil {
		walletName = m.wallet.GetFileName()
		if walletName == "" && m.walletFile != "" {
			walletName = filepath.Base(m.walletFile)
		}
	}

	// Get balance info if wallet is open
	var balanceStr string
	if m.wallet != nil {
		info := m.wallet.GetInfo()
		balance := float64(info.Balance) / 100000.0
		balanceStr = fmt.Sprintf("%s %.5f DERO", styles.BalanceGlyph, balance)
	}

	switch m.page {
	case PageWelcome:
		return base + " - Welcome"
	case PageFilePicker:
		return base + " - Select Wallet"
	case PagePassword:
		if m.isCreating {
			return base + " - Create Wallet"
		} else if m.isRestoringFromSeed {
			return base + " - Restore from Seed"
		} else if m.isRestoringFromKey {
			return base + " - Restore from Key"
		} else if m.isChangingPassword {
			if walletName != "" {
				return base + " - Change Password - " + walletName
			}
			return base + " - Change Password"
		} else {
			if walletName != "" {
				return base + " - Unlock - " + walletName
			}
			return base + " - Unlock Wallet"
		}
	case PageNetwork:
		return base + " - Select Network"
	case PageSeed:
		if m.isRestoringFromSeed {
			return base + " - Enter Seed"
		}
		if walletName != "" {
			return base + " - Seed Phrase - " + walletName
		}
		return base + " - Seed Phrase"
	case PageKeyInput:
		if m.isRestoringFromKey {
			return base + " - Enter Hex Key"
		}
		if walletName != "" {
			return base + " - Hex Key - " + walletName
		}
		return base + " - Hex Key"
	case PageQRCode:
		if walletName != "" {
			return base + " - QR Code - " + walletName
		}
		return base + " - Address QR Code"
	case PageMain:
		if walletName != "" && balanceStr != "" {
			syncStatus := "Offline"
			if m.wallet != nil {
				info := m.wallet.GetInfo()
				if info.IsOnline {
					if info.IsSynced {
						syncStatus = "Synced"
					} else {
						syncStatus = "Syncing"
					}
				}
			}
			return base + " - " + walletName + " - " + balanceStr + " [" + syncStatus + "]"
		}
		return base + " - Dashboard"
	case PageSend:
		if balanceStr != "" {
			return base + " - Send [" + balanceStr + " available]"
		}
		return base + " - Send DERO"
	case PageHistory:
		if walletName != "" {
			return base + " - History - " + walletName
		}
		return base + " - Transaction History"
	case PageTxDetails:
		return base + " - Transaction Details"
	case PageDaemon:
		return base + " - Connect Daemon"
	case PageIntegratedAddr:
		if walletName != "" {
			return base + " - Payment Request - " + walletName
		}
		return base + " - Payment Request"
	case PageXSWDAuth:
		return base + " - XSWD Authorization"
	case PageXSWDPerm:
		return base + " - XSWD Permission"
	default:
		return base
	}
}
