// Copyright 2017-2026 DERO Project. All rights reserved.

package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// Plan is a preview-only installer plan.
type Plan struct {
	Mode              string
	ServiceType       string
	ServiceScope      string
	FallbackNote      string
	DownloadSource    string
	ReleaseTag        string
	AssetName         string
	AssetURL          string
	AssetType         string
	BinaryTarget      string
	BinaryReady       bool
	UnitTarget        string
	ExecStart         string
	Network           string
	DataDir           string
	RPCBind           string
	P2PBind           string
	GetWorkBind       string
	NodeTag           string
	IntegratorAddress string
	StartNow          bool
}

// BuildBuiltinServicePlan builds a plan that registers the built-in daemon
// (derotui daemon-helper --service) as a systemd user service. No external
// derod binary is downloaded — the daemon is the same one embedded in the
// TUI, running as a background service.
func BuildBuiltinServicePlan(settings config.DaemonSettings) (Plan, error) {
	exe, err := os.Executable()
	if err != nil {
		return Plan{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Plan{}, err
	}
	unitTarget := filepath.Join(home, ".config", "systemd", "user", "derod.service")
	execStart := fmt.Sprintf("%s daemon-helper --service", exe)
	return Plan{
		Mode:              "service",
		ServiceType:       "User Service (built-in daemon)",
		ServiceScope:      "user",
		FallbackNote:      "Runs the same daemon embedded in derotui as a background service. No download needed.",
		BinaryTarget:      exe,
		BinaryReady:       true,
		UnitTarget:        unitTarget,
		ExecStart:         execStart,
		Network:           settings.Network,
		DataDir:           settings.DataDir,
		RPCBind:           settings.RPCBind,
		P2PBind:           settings.P2PBind,
		GetWorkBind:       settings.GetWorkBind,
		NodeTag:           settings.NodeTag,
		IntegratorAddress: settings.IntegratorAddress,
		StartNow:          true,
	}, nil
}

// WithUserServiceFallback returns a user-service variant of the plan.
func WithUserServiceFallback(plan Plan) Plan {
	home, err := os.UserHomeDir()
	if err == nil {
		plan.UnitTarget = filepath.Join(home, ".config", "systemd", "user", "derod.service")
	}
	plan.ServiceType = "User Service"
	plan.ServiceScope = "user"
	plan.FallbackNote = "User service avoids system-level writes. For true always-on behavior without login, enable linger manually."
	return plan
}
