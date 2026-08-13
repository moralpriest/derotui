// Copyright 2017-2026 DERO Project. All rights reserved.

package systemd

import (
	"fmt"
	"os/exec"
	"strings"
)

// Scope identifies a systemd manager scope.
type Scope string

const (
	ScopeNone   Scope = ""
	ScopeUser   Scope = "user"
	ScopeSystem Scope = "system"
)

// ServiceStatus describes a detected systemd service.
type ServiceStatus struct {
	Unit         string
	Scope        Scope
	Exists       bool
	Active       bool
	Enabled      bool
	SubState     string
	LoadState    string
	RawError     string
	FragmentPath string
	ExecStart    string
}

// DetectionResult describes user/system unit presence for the same service name.
type DetectionResult struct {
	User   ServiceStatus
	System ServiceStatus
}

// Detect checks whether a systemd unit exists in user or system scope.
func Detect(unit string) (ServiceStatus, error) {
	if status, ok := inspect(unit, ScopeUser); ok {
		return status, nil
	}
	if status, ok := inspect(unit, ScopeSystem); ok {
		return status, nil
	}
	return ServiceStatus{Unit: unit, Scope: ScopeNone}, nil
}

// DetectAll checks user and system scopes without preferring either.
func DetectAll(unit string) (DetectionResult, error) {
	var result DetectionResult
	if status, ok := inspect(unit, ScopeUser); ok {
		result.User = status
	}
	if status, ok := inspect(unit, ScopeSystem); ok {
		result.System = status
	}
	return result, nil
}

func inspect(unit string, scope Scope) (ServiceStatus, bool) {
	showArgs := []string{"show", unit, "--property=LoadState,ActiveState,SubState,UnitFileState,FragmentPath,ExecStart", "--value"}
	cmd := exec.Command("systemctl", withScope(scope, showArgs)...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil && !strings.Contains(text, "not-found") && !strings.Contains(strings.ToLower(text), "could not be found") {
		return ServiceStatus{Unit: unit, Scope: scope, RawError: text}, false
	}
	parts := strings.Split(text, "\n")
	if len(parts) < 6 || strings.TrimSpace(parts[0]) == "not-found" || strings.TrimSpace(parts[0]) == "" {
		return ServiceStatus{}, false
	}
	status := ServiceStatus{
		Unit:         unit,
		Scope:        scope,
		Exists:       true,
		LoadState:    strings.TrimSpace(parts[0]),
		Active:       strings.TrimSpace(parts[1]) == "active",
		SubState:     strings.TrimSpace(parts[2]),
		Enabled:      strings.TrimSpace(parts[3]) == "enabled",
		FragmentPath: strings.TrimSpace(parts[4]),
		ExecStart:    strings.TrimSpace(parts[5]),
	}
	return status, true
}

// Control runs a lifecycle action against a detected service.
func Control(scope Scope, unit, action string) error {
	valid := map[string]bool{"start": true, "stop": true, "restart": true}
	if !valid[action] {
		return fmt.Errorf("unsupported systemd action %q", action)
	}
	cmd := exec.Command("systemctl", withScope(scope, []string{action, unit})...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// Journal reads recent journal lines for a unit.
func Journal(scope Scope, unit string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 100
	}
	cmd := exec.Command("journalctl", withScope(scope, []string{"-u", unit, "-n", fmt.Sprintf("%d", lines), "--no-pager"})...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	raw := strings.Split(text, "\n")
	result := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func withScope(scope Scope, args []string) []string {
	if scope == ScopeUser {
		return append([]string{"--user"}, args...)
	}
	return args
}
