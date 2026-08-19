// Copyright 2017-2026 DERO Project. All rights reserved.

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deroproject/dero-wallet-cli/internal/config"
	"github.com/deroproject/dero-wallet-cli/internal/services/releases"
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

// BuildPlan builds a preview-only system service install plan.
func BuildPlan(settings config.DaemonSettings, match releases.Match) (Plan, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Plan{}, err
	}
	binaryTarget := settings.BinaryPath
	if strings.TrimSpace(binaryTarget) == "" {
		binaryTarget = filepath.Join(home, ".derotui", "derod")
	}
	unitTarget := "/etc/systemd/system/derod.service"
	execStart := fmt.Sprintf("%s --rpc-bind=%s --p2p-bind=%s --getwork-bind=%s --data-dir=%s", binaryTarget, settings.RPCBind, settings.P2PBind, settings.GetWorkBind, settings.DataDir)
	if settings.FastSync {
		hasData := false
		if entries, err := os.ReadDir(settings.DataDir); err == nil {
			hasData = len(entries) > 0
		}
		if !hasData {
			execStart += " --fastsync"
		}
	}
	if settings.Debug {
		execStart += " --debug"
	}
	if settings.IntegratorAddress != "" {
		execStart += " --integrator-address=" + settings.IntegratorAddress
	}
	if settings.NodeTag != "" {
		execStart += " --node-tag=" + settings.NodeTag
	}
	if settings.Network == string(config.NetworkTestnet) {
		execStart += " --testnet"
	}
	_, statErr := os.Stat(binaryTarget)
	return Plan{
		Mode:              "service",
		ServiceType:       "System Service (Recommended)",
		ServiceScope:      "system",
		FallbackNote:      "If system service install later fails, derotui can fall back to a user service.",
		DownloadSource:    settings.DownloadSource,
		ReleaseTag:        match.TagName,
		AssetName:         match.Asset.Name,
		AssetURL:          match.Asset.DownloadURL,
		AssetType:         assetType(match.Asset.Name),
		BinaryTarget:      binaryTarget,
		BinaryReady:       statErr == nil,
		UnitTarget:        unitTarget,
		ExecStart:         execStart,
		Network:           settings.Network,
		DataDir:           settings.DataDir,
		RPCBind:           settings.RPCBind,
		P2PBind:           settings.P2PBind,
		GetWorkBind:       settings.GetWorkBind,
		NodeTag:           settings.NodeTag,
		IntegratorAddress: settings.IntegratorAddress,
		StartNow:          false,
	}, nil
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

// BuildSessionPlan builds a preview-only session start plan.
func BuildSessionPlan(settings config.DaemonSettings, match releases.Match) (Plan, error) {
	plan, err := BuildPlan(settings, match)
	if err != nil {
		return Plan{}, err
	}
	plan.Mode = "session"
	plan.ServiceType = "Session Daemon"
	plan.ServiceScope = ""
	plan.UnitTarget = ""
	plan.FallbackNote = "The binary can be downloaded and then started directly without installing a service."
	return plan, nil
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

func assetType(name string) string {
	name = strings.ToLower(name)
	if strings.HasSuffix(name, ".zip") {
		return "zip archive"
	}
	if strings.HasSuffix(name, ".tar.gz") {
		return "tar.gz archive"
	}
	return "archive"
}
