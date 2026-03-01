// Copyright 2017-2026 DERO Project. All rights reserved.

package pages

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestHistoryMouseSingleThenDoubleClick(t *testing.T) {
	h := NewHistory()
	h.SetTransactions([]Transaction{{TxID: "tx1"}, {TxID: "tx2"}})

	// Coordinates chosen to land on first row in HandleMouse calculations.
	msg := tea.MouseClickMsg(tea.Mouse{Button: tea.MouseLeft, X: 10, Y: 7})
	h = h.HandleMouse(msg, 80, 24)

	if h.cursor != 0 {
		t.Fatalf("expected cursor to select first row, got %d", h.cursor)
	}
	if h.wantDetails {
		t.Fatalf("expected single click not to open details")
	}

	// Second click on same row within threshold should open details.
	h = h.HandleMouse(msg, 80, 24)
	if !h.wantDetails {
		t.Fatalf("expected double click to open details")
	}
}

func TestHistoryMouseClickTimeoutDoesNotOpenDetails(t *testing.T) {
	h := NewHistory()
	h.SetTransactions([]Transaction{{TxID: "tx1"}})

	msg := tea.MouseClickMsg(tea.Mouse{Button: tea.MouseLeft, X: 10, Y: 7})
	h = h.HandleMouse(msg, 80, 24)

	h.wantDetails = false
	h.lastClickAt = time.Now().Add(-time.Second)
	h = h.HandleMouse(msg, 80, 24)

	if h.wantDetails {
		t.Fatalf("expected second click after timeout not to open details")
	}
}
