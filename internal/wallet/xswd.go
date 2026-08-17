// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"fmt"
	"time"

	"github.com/civilware/epoch"
	"github.com/creachadair/jrpc2"
	"github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/xswd"
)

// XSWDDialogTimeout is how long the TUI may show an auth/permission dialog
// before the server-side handler times out and denies the request.
const XSWDDialogTimeout = 30 * time.Second

type XSWDAppInfo struct {
	Name        string
	Description string
	URL         string
	ID          string
}

type XSWDPermRequest struct {
	AppName string
	Method  string
}

const (
	XSWDPermAsk         = 0
	XSWDPermAllow       = 1
	XSWDPermDeny        = 2
	XSWDPermAlwaysAllow = 3
	XSWDPermAlwaysDeny  = 4
)

// XSWDBridge wraps the XSWD server and provides a clean interface for the TUI.
// It encapsulates all xswd and jrpc2 imports so the UI layer doesn't need them.
type XSWDBridge struct {
	server       *xswd.XSWD
	epochRunning bool
}

// EpochRunning reports whether the EPOCH GetWork connection is active.
func (b *XSWDBridge) EpochRunning() bool {
	return b != nil && b.epochRunning
}

type MsgSender interface {
	Send(msg interface{})
}

type XSWDAuthRequest struct {
	App      XSWDAppInfo
	Response chan bool
}

type XSWDPermissionRequest struct {
	Perm     XSWDPermRequest
	Response chan int
}

type XSWDStartedMsg struct {
	Bridge *XSWDBridge
	Err    error
}

// epochMethods is the set of EPOCH methods exposed over XSWD. These are
// auto-allowed so TELA dApps can crowd-mine without a permission dialog on
// every attempt, while sensitive wallet methods (transfer, query_key, ...)
// still require explicit user approval.
var epochMethods = map[string]bool{
	"AttemptEPOCH":      true,
	"SubmitEPOCH":       true,
	"GetMaxHashesEPOCH": true,
	"GetAddressEPOCH":   true,
	"GetSessionEPOCH":   true,
	"StopEPOCH":         true,
}

// isEpochMethod reports whether the given XSWD method belongs to the EPOCH
// crowd-mining API and can be auto-allowed.
func isEpochMethod(method string) bool {
	return epochMethods[method]
}

// startEpoch registers the EPOCH handlers on the XSWD server and connects to
// the daemon's GetWork endpoint. It returns the XSWD server so the caller can
// keep starting it even when EPOCH fails (the TELA dApp still works, just
// without crowd-mining). Errors are logged and surfaced via the returned error.
func startEpoch(server *xswd.XSWD, rewardAddress, daemonAddress, network string) error {
	for method, fn := range epoch.GetHandler() {
		server.SetCustomMethod(method, fn)
	}

	if daemonAddress == "" || daemonAddress == "Not connected" {
		return fmt.Errorf("daemon not connected")
	}

	switch network {
	case "Testnet":
		_ = epoch.SetPort(40400)
	case "Simulator":
		_ = epoch.SetPort(20000)
	default:
		_ = epoch.SetPort(10100)
	}

	if err := epoch.StartGetWork(rewardAddress, daemonAddress); err != nil {
		return fmt.Errorf("epoch: %w", err)
	}
	log.Info("epoch", "start", "EPOCH GetWork started", "daemon", daemonAddress, "network", network)
	return nil
}

// StartXSWD creates and starts the XSWD server, returning a bridge.
// The sender is used to inject auth/perm request messages into the TUI event loop.
// This function blocks briefly while starting the HTTP server goroutine, then returns.
// EPOCH is started alongside XSWD so TELA dApps can crowd-mine via the same server.
func StartXSWD(w *walletapi.Wallet_Disk, sender MsgSender, rewardAddress, daemonAddress, network string) *XSWDBridge {
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
			case <-time.After(XSWDDialogTimeout):
				log.Warn("xswd", "auth.timeout", "Authorization request timed out", "app", app.Name)
				return false
			}
		},
		// requestHandler - called when a dApp method needs permission
		func(app *xswd.ApplicationData, req *jrpc2.Request) xswd.Permission {
			// EPOCH crowd-mining methods are non-sensitive and cheap; auto-allow
			// them so a TELA dApp does not trigger a permission dialog per attempt.
			if isEpochMethod(req.Method()) {
				return xswd.AlwaysAllow
			}

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
			case <-time.After(XSWDDialogTimeout):
				log.Warn("xswd", "perm.timeout", "Permission request timed out", "app", app.Name, "method", req.Method())
				return xswd.Deny
			}
		},
	)

	bridge.server = server
	if err := startEpoch(server, rewardAddress, daemonAddress, network); err != nil {
		log.Warn("epoch", "start.failed", "EPOCH not started", "error", err.Error())
	} else {
		bridge.epochRunning = true
	}
	log.Info("xswd", "server.started", "XSWD server started", "port", "44326")
	return bridge
}

// Stop stops the XSWD server, the EPOCH GetWork connection, and cleans up.
func (b *XSWDBridge) Stop() {
	if b.server != nil && b.server.IsRunning() {
		log.Info("xswd", "server.stopped", "XSWD server stopped")
		b.server.Stop()
		b.server = nil
	}
	if b.epochRunning {
		epoch.StopGetWork()
		b.epochRunning = false
		log.Info("epoch", "stop", "EPOCH GetWork stopped")
	}
}

func (b *XSWDBridge) IsRunning() bool {
	return b != nil && b.server != nil && b.server.IsRunning()
}

func XSWDUnavailableError() error {
	return fmt.Errorf("XSWD is temporarily unavailable with the current embedded daemon dependency set")
}
