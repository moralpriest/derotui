// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/xswd"
)

const xswdDialogTimeout = 30 * time.Second

// XSWDAppInfo contains dApp info for the TUI authorization dialog
type XSWDAppInfo struct {
	Name        string
	Description string
	URL         string
	ID          string
}

// XSWDPermRequest contains permission request info for the TUI dialog
type XSWDPermRequest struct {
	AppName string
	Method  string
}

// XSWD permission values matching xswd.Permission enum
const (
	XSWDPermAsk         = int(xswd.Ask)         // 0
	XSWDPermAllow       = int(xswd.Allow)       // 1
	XSWDPermDeny        = int(xswd.Deny)        // 2
	XSWDPermAlwaysAllow = int(xswd.AlwaysAllow) // 3
	XSWDPermAlwaysDeny  = int(xswd.AlwaysDeny)  // 4
)

// XSWDBridge wraps the XSWD server and provides a clean interface for the TUI.
// It encapsulates all xswd and jrpc2 imports so the UI layer doesn't need them.
type XSWDBridge struct {
	server *xswd.XSWD
}

// MsgSender is the interface used to send messages into the TUI event loop.
// In practice this is *tea.Program but we use an interface to avoid importing
// bubbletea in the wallet package.
type MsgSender interface {
	Send(msg interface{})
}

// XSWDAuthRequest is sent to the TUI when a dApp requests authorization.
// The TUI must send true/false on the Response channel.
type XSWDAuthRequest struct {
	App      XSWDAppInfo
	Response chan bool
}

// XSWDPermissionRequest is sent to the TUI when a dApp method needs permission.
// The TUI must send an int (XSWDPermAllow..XSWDPermAlwaysDeny) on the Response channel.
type XSWDPermissionRequest struct {
	Perm     XSWDPermRequest
	Response chan int
}

// XSWDStartedMsg is sent to the TUI when the XSWD server starts or fails.
type XSWDStartedMsg struct {
	Bridge *XSWDBridge
	Err    error
}

// StartXSWD creates and starts the XSWD server, returning a bridge.
// The sender is used to inject auth/perm request messages into the TUI event loop.
// This function blocks briefly while starting the HTTP server goroutine, then returns.
func StartXSWD(w *walletapi.Wallet_Disk, sender MsgSender) *XSWDBridge {
	bridge := &XSWDBridge{}

	server := xswd.NewXSWDServer(w,
		// appHandler - called when a dApp connects and needs authorization
		func(app *xswd.ApplicationData) bool {
			ch := make(chan bool, 1)
			sender.Send(XSWDAuthRequest{
				App: XSWDAppInfo{
					Name:        app.Name,
					Description: app.Description,
					URL:         app.Url,
					ID:          app.Id,
				},
				Response: ch,
			})
			select {
			case result := <-ch:
				return result
			case <-app.OnClose:
				return false
			case <-time.After(xswdDialogTimeout):
				log.Warn("xswd", "auth.timeout", "Authorization request timed out", "app", app.Name)
				return false
			}
		},
		// requestHandler - called when a dApp method needs permission
		func(app *xswd.ApplicationData, req *jrpc2.Request) xswd.Permission {
			ch := make(chan int, 1)
			sender.Send(XSWDPermissionRequest{
				Perm: XSWDPermRequest{
					AppName: app.Name,
					Method:  req.Method(),
				},
				Response: ch,
			})
			select {
			case result := <-ch:
				return xswd.Permission(result)
			case <-app.OnClose:
				return xswd.Deny
			case <-time.After(xswdDialogTimeout):
				log.Warn("xswd", "perm.timeout", "Permission request timed out", "app", app.Name, "method", req.Method())
				return xswd.Deny
			}
		},
	)

	bridge.server = server
	log.Info("xswd", "server.started", "XSWD server started", "port", "44326")
	return bridge
}

// Stop stops the XSWD server and cleans up.
func (b *XSWDBridge) Stop() {
	if b.server != nil && b.server.IsRunning() {
		log.Info("xswd", "server.stopped", "XSWD server stopped")
		b.server.Stop()
		b.server = nil
	}
}

// IsRunning returns true if the XSWD server is currently running.
func (b *XSWDBridge) IsRunning() bool {
	return b.server != nil && b.server.IsRunning()
}
