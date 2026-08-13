// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"fmt"
	"time"

	"github.com/deroproject/derohe/walletapi"
)

const xswdDialogTimeout = 30 * time.Second

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

type XSWDBridge struct{}

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

func StartXSWD(_ *walletapi.Wallet_Disk, _ MsgSender) *XSWDBridge {
	return nil
}

func (b *XSWDBridge) Stop() {}

func (b *XSWDBridge) IsRunning() bool {
	return false
}

func XSWDUnavailableError() error {
	return fmt.Errorf("XSWD is temporarily unavailable with the current embedded daemon dependency set")
}
