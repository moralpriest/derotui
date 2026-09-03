// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/civilware/epoch"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/log"
	"github.com/deroproject/derohe/rpc"
	"github.com/deroproject/derohe/walletapi"
	"github.com/deroproject/derohe/walletapi/xswd"
	hgstorage "github.com/hypergnomon/hypergnomon/pkg/gnomes/storage"
	hgstruct "github.com/hypergnomon/hypergnomon/pkg/gnomes/structures"
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
	epochErr     error
}

// EpochRunning reports whether the EPOCH GetWork connection is active.
func (b *XSWDBridge) EpochRunning() bool {
	return b != nil && b.epochRunning
}

// EpochError is the GetWork connect failure, if any.
func (b *XSWDBridge) EpochError() error {
	if b == nil {
		return nil
	}
	return b.epochErr
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
	"AttemptEPOCH":         true,
	"AttemptEPOCHWithAddr": true,
	"SubmitEPOCH":          true,
	"GetMaxHashesEPOCH":    true,
	"GetAddressEPOCH":      true,
	"GetSessionEPOCH":      true,
	"StopEPOCH":            true,
}

type attemptWithAddrParams struct {
	Hashes  int    `json:"hashes"`
	Address string `json:"address"`
}

// isEpochMethod reports whether the given XSWD method belongs to the EPOCH
// crowd-mining API and can be auto-allowed.
func isEpochMethod(method string) bool {
	return epochMethods[method]
}

func isGnomonMethod(method string) bool {
	return strings.HasPrefix(method, "Gnomon.")
}

func isDeroMethod(method string) bool {
	return strings.HasPrefix(method, "DERO.") || method == "GetSC"
}

type gnomonSCIDParam struct {
	SCID string `json:"scid"`
}

type gnomonAllVarsResult struct {
	AllVariables []*hgstruct.SCIDVariable `json:"allVariables"`
}

func appHyperStore() *hgstorage.BboltStore {
	globalAppHyperMu.RLock()
	h := globalAppHyper
	globalAppHyperMu.RUnlock()
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.store
}

func gnomonGetAllSCIDVariableDetails(_ context.Context, p gnomonSCIDParam) (gnomonAllVarsResult, error) {
	globalAppHyperMu.RLock()
	h := globalAppHyper
	globalAppHyperMu.RUnlock()
	if h == nil {
		return gnomonAllVarsResult{}, fmt.Errorf("gnomon is not active")
	}
	scid := strings.ToLower(strings.TrimSpace(p.SCID))
	var vars []*hgstruct.SCIDVariable
	if store := appHyperStore(); store != nil {
		vars = store.GetSCIDVariableDetailsAtTopoheight(scid, 0)
	}
	if len(vars) == 0 {
		vars = liveSCVariables(h.Endpoint(), scid)
	}
	return gnomonAllVarsResult{AllVariables: vars}, nil
}

func registerGnomon(server *xswd.XSWD) {
	server.SetCustomMethod("Gnomon.GetAllSCIDVariableDetails", handler.New(gnomonGetAllSCIDVariableDetails))
}

func registerDeroGetSC(server *xswd.XSWD, daemonAddress string) {
	fn := handler.New(func(_ context.Context, p rpc.GetSC_Params) (rpc.GetSC_Result, error) {
		return daemonGetSC(daemonAddress, p)
	})
	server.SetCustomMethod("DERO.GetSC", fn)
	server.SetCustomMethod("GetSC", fn)
}

// startEpoch registers the EPOCH handlers on the XSWD server and connects to
// the daemon's GetWork endpoint. It returns the XSWD server so the caller can
// keep starting it even when EPOCH fails (the TELA dApp still works, just
// without crowd-mining). Errors are logged and surfaced via the returned error.
func epochGetWorkPort(network, getWorkBind string) int {
	if _, port, err := net.SplitHostPort(strings.TrimSpace(getWorkBind)); err == nil {
		if p, err := strconv.Atoi(port); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	switch strings.ToLower(strings.TrimSpace(network)) {
	case "testnet":
		return 40400
	case "simulator":
		return 20003
	default:
		return 10100
	}
}

func attemptEPOCHWithAddr(ctx context.Context, p attemptWithAddrParams, w *walletapi.Wallet_Disk, daemonHP string) (epoch.EPOCH_Result, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return epoch.EPOCH_Result{}, err
		}
	}
	if strings.TrimSpace(p.Address) == "" {
		return epoch.EPOCH_Result{}, fmt.Errorf("address param cannot be empty")
	}
	addr := strings.TrimSpace(p.Address)
	if len(addr) < 66 && w != nil {
		resolved, err := w.NameToAddress(addr)
		if err != nil {
			return epoch.EPOCH_Result{}, fmt.Errorf("could not match name to DERO address: %w", err)
		}
		addr = resolved
	}
	if p.Hashes > epoch.GetMaxHashes() {
		return epoch.EPOCH_Result{}, fmt.Errorf("hashes exceeds maxHashes %d/%d", p.Hashes, epoch.GetMaxHashes())
	}
	if daemonHP == "" {
		return epoch.EPOCH_Result{}, fmt.Errorf("daemon not connected")
	}
	if !epoch.IsActive() || epoch.GetAddress() != addr {
		if epoch.IsActive() {
			epoch.StopGetWork()
		}
		if err := epoch.StartGetWork(addr, daemonHP); err != nil {
			return epoch.EPOCH_Result{}, err
		}
		if err := epoch.JobIsReady(10 * time.Second); err != nil {
			return epoch.EPOCH_Result{}, err
		}
	}
	return epoch.AttemptHashes(p.Hashes)
}

func startEpoch(server *xswd.XSWD, w *walletapi.Wallet_Disk, rewardAddress, daemonAddress, network string) error {
	for method, fn := range epoch.GetHandler() {
		server.SetCustomMethod(method, fn)
	}
	hp, _ := daemonHostPort(daemonAddress)
	server.SetCustomMethod("AttemptEPOCHWithAddr", handler.New(func(ctx context.Context, p attemptWithAddrParams) (epoch.EPOCH_Result, error) {
		return attemptEPOCHWithAddr(ctx, p, w, hp)
	}))

	if daemonAddress == "" || daemonAddress == "Not connected" {
		return fmt.Errorf("daemon not connected")
	}
	if hp == "" {
		var err error
		hp, err = daemonHostPort(daemonAddress)
		if err != nil {
			return err
		}
	}
	port := epochGetWorkPort(network, config.GetDaemonSettings().GetWorkBind)
	if err := epoch.SetPort(port); err != nil {
		return err
	}

	if err := epoch.StartGetWork(rewardAddress, hp); err != nil {
		return fmt.Errorf("epoch: %w", err)
	}
	log.Info("epoch", "start", "EPOCH GetWork started", "daemon", hp, "getwork", strconv.Itoa(port), "network", network)
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
			if isEpochMethod(req.Method()) || isGnomonMethod(req.Method()) || isDeroMethod(req.Method()) {
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
	registerGnomon(server)
	registerDeroGetSC(server, daemonAddress)
	if err := startEpoch(server, w, rewardAddress, daemonAddress, network); err != nil {
		bridge.epochErr = err
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
