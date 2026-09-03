// Copyright 2017-2026 DERO Project. All rights reserved.

package wallet

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/civilware/tela"
	"github.com/deroproject/dero-wallet-cli/internal/log"
)

// NOTE: no localhost rewrite here. XSWD sameWebOrigin (derohe dev) treats
// localhost/127.0.0.1/::1 as one origin, so TELA's 127.0.0.1 links match
// dApps that declare http://localhost:PORT.

func existingTelaLink(scid string) string {
	for _, s := range tela.GetServerInfo() {
		if !strings.EqualFold(s.SCID, scid) {
			continue
		}
		if s.Entrypoint == "" {
			return "http://" + s.Address
		}
		return "http://" + s.Address + "/" + s.Entrypoint
	}
	return ""
}

// ServeTela clones a TELA-INDEX and serves it, returning the local URL.
// If a server for scid is already running, it returns that URL.
func ServeTela(scid, endpoint string, cancelled *atomic.Bool) (string, error) {
	scid = strings.TrimSpace(scid)
	if len(scid) != 64 {
		return "", fmt.Errorf("invalid SCID")
	}
	if cancelled != nil && cancelled.Load() {
		return "", fmt.Errorf("cancelled")
	}
	if link := existingTelaLink(scid); link != "" {
		return link, nil
	}
	tela.AllowUpdates(true)
	defer tela.AllowUpdates(false)
	var extra []*atomic.Bool
	if cancelled != nil {
		extra = []*atomic.Bool{cancelled}
	}
	link, err := tela.OpenTELALink("tela://open/"+scid, endpoint, extra...)
	if err != nil {
		return "", err
	}
	log.Info("tela", "serve", "TELA server started", "link", link, "scid", scid)
	return link, nil
}

// ShutdownTela stops every TELA server and removes cloned files.
func ShutdownTela() {
	tela.ShutdownTELA()
}

func browserCmd(goos, link string) (name string, args []string) {
	switch goos {
	case "linux", "freebsd", "netbsd", "openbsd":
		return "xdg-open", []string{link}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", link}
	case "darwin":
		return "open", []string{link}
	}
	return "", nil
}

// OpenBrowser opens link in the OS default browser.
func OpenBrowser(link string) error {
	name, args := browserCmd(runtime.GOOS, link)
	if name == "" {
		return fmt.Errorf("no browser opener for %s", runtime.GOOS)
	}
	return exec.Command(name, args...).Start()
}
