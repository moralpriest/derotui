// Copyright 2017-2026 DERO Project. All rights reserved.

package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// handleMouseEvent routes mouse events to the appropriate page.
func (m *Model) handleMouseEvent(msg tea.MouseMsg) bool {
	if m.handleDebugMouse(msg) {
		return true
	}

	// Type assert to MouseClickMsg for page handlers
	clickMsg, ok := msg.(tea.MouseClickMsg)
	if !ok {
		return false
	}

	switch m.page {
	case PageWelcome:
		return m.welcome.HandleMouse(clickMsg, m.width, m.height)
	case PageMain:
		// Dashboard handles mouse within its own update
		m.dashboard = m.dashboard.HandleMouse(clickMsg, m.width, m.height)
		return false // Dashboard returns modified model, we handle in main Update
	case PageSend:
		m.send = m.send.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageHistory:
		m.history = m.history.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageTxDetails:
		m.txDetails = m.txDetails.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PagePassword:
		m.password = m.password.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageSeed:
		m.seed = m.seed.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageKeyInput:
		m.keyInput = m.keyInput.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageDaemon:
		m.daemon = m.daemon.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageIntegratedAddr:
		m.integratedAddr = m.integratedAddr.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageNetwork:
		m.network = m.network.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageQRCode:
		m.qrcode = m.qrcode.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageXSWDAuth:
		m.xswdAuth = m.xswdAuth.HandleMouse(clickMsg, m.width, m.height)
		return false
	case PageXSWDPerm:
		m.xswdPerm = m.xswdPerm.HandleMouse(clickMsg, m.width, m.height)
		return false
	}
	return false
}

func (m *Model) handleDebugMouse(msg tea.MouseMsg) bool {
	if !m.debugEnabled || !m.debugConsoleOpen {
		return false
	}

	width, height := m.width, m.height
	if width == 0 {
		width = 100
	}
	if height == 0 {
		height = 40
	}

	logLines := debugExpandedLogLines
	consoleHeight := logLines + 4
	consoleTop := height - consoleHeight
	if consoleTop < 0 {
		consoleTop = 0
	}

	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouseY := msg.Mouse().Y
		// Only capture clicks inside debug panel area
		if mouseY < consoleTop {
			return false
		}
		maxStart := m.maxDebugScrollStart(logLines)

		// Click inside debug panel switches to manual scroll mode.
		// Double-click near the bottom snaps back to tail-follow.
		bottomAreaTop := height - 2
		now := time.Now()
		doubleClick := (m.debugLastClickY == mouseY) && now.Sub(m.debugLastClickAt) < 450*time.Millisecond
		m.debugLastClickY = mouseY
		m.debugLastClickAt = now

		if mouseY >= bottomAreaTop && doubleClick {
			m.debugAutoFollow = true
			m.debugScrollStart = maxStart
			return true
		}

		m.debugAutoFollow = false
		return true
	case tea.MouseWheelMsg:
		if logLines <= 0 {
			return true
		}
		mouseY := msg.Mouse().Y
		// Only capture wheel events inside debug panel area
		if mouseY < consoleTop {
			return false
		}
		maxStart := m.maxDebugScrollStart(logLines)

		switch msg.Button {
		case tea.MouseWheelUp:
			m.debugAutoFollow = false
			m.debugScrollStart--
			if m.debugScrollStart < 0 {
				m.debugScrollStart = 0
			}
			return true
		case tea.MouseWheelDown:
			m.debugScrollStart++
			if m.debugScrollStart >= maxStart {
				m.debugScrollStart = maxStart
				m.debugAutoFollow = true
			}
			return true
		}
	}

	return false
}
