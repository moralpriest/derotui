// Copyright 2017-2026 DERO Project. All rights reserved.

package installer

import (
	"strings"
	"testing"

	"github.com/deroproject/dero-wallet-cli/internal/config"
)

// BuildBuiltinServicePlan registers the built-in daemon as a user service —
// no external binary download, and the ExecStart runs the current executable
// in daemon-helper service mode.
func TestBuildBuiltinServicePlan(t *testing.T) {
	settings := config.DefaultDaemonSettingsForNetwork("mainnet")
	plan, err := BuildBuiltinServicePlan(settings)
	if err != nil {
		t.Fatalf("BuildBuiltinServicePlan: %v", err)
	}

	if plan.Mode != "service" {
		t.Errorf("Mode = %q, want service", plan.Mode)
	}
	if plan.ServiceScope != "user" {
		t.Errorf("ServiceScope = %q, want user", plan.ServiceScope)
	}
	if !plan.BinaryReady {
		t.Error("BinaryReady = false, want true (built-in binary needs no download)")
	}
	if plan.ReleaseTag != "" {
		t.Errorf("ReleaseTag = %q, want empty (no download)", plan.ReleaseTag)
	}
	if !strings.Contains(plan.ExecStart, " daemon-helper --service") {
		t.Errorf("ExecStart = %q, want it to run daemon-helper --service", plan.ExecStart)
	}
	if !strings.HasSuffix(plan.UnitTarget, ".config/systemd/user/derod.service") {
		t.Errorf("UnitTarget = %q, want a user-scope unit", plan.UnitTarget)
	}
	if plan.DataDir == "" || plan.RPCBind == "" || plan.P2PBind == "" || plan.GetWorkBind == "" {
		t.Error("plan should carry data dir and binds from settings")
	}
}
