// Copyright 2017-2026 DERO Project. All rights reserved.

package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InstallUnit installs a systemd unit file and reloads the daemon.
func InstallUnit(scope Scope, unitPath, unitContent string) error {
	return installUnit(scope, unitPath, unitContent, false)
}

// InstallUnitWithSudo installs a system unit using sudo explicitly.
func InstallUnitWithSudo(unitPath, unitContent string) error {
	return installUnit(ScopeSystem, unitPath, unitContent, true)
}

// EnableUnit enables a unit in systemd.
func EnableUnit(scope Scope, unit string) error {
	return runSystemctl(scope, false, "enable", unit)
}

// EnableUnitWithSudo enables a system unit using sudo.
func EnableUnitWithSudo(unit string) error {
	return runSystemctl(ScopeSystem, true, "enable", unit)
}

// StartUnit starts a unit in systemd.
func StartUnit(scope Scope, unit string) error {
	return runSystemctl(scope, false, "start", unit)
}

// StartUnitWithSudo starts a system unit using sudo.
func StartUnitWithSudo(unit string) error {
	return runSystemctl(ScopeSystem, true, "start", unit)
}

func installUnit(scope Scope, unitPath, unitContent string, useSudo bool) error {
	if scope != ScopeSystem && scope != ScopeUser {
		return fmt.Errorf("unsupported install scope")
	}
	if useSudo && scope != ScopeSystem {
		return fmt.Errorf("sudo install is only supported for system scope")
	}
	if scope == ScopeUser {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		unitPath = filepath.Join(home, ".config", "systemd", "user", filepath.Base(unitPath))
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return err
	}
	if useSudo {
		writeCmd := exec.Command("sudo", "tee", unitPath)
		writeCmd.Stdin = strings.NewReader(unitContent)
		out, err := writeCmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("%s", msg)
		}
		chmodCmd := exec.Command("sudo", "chmod", "0644", unitPath)
		if out, err := chmodCmd.CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			return fmt.Errorf("%s", msg)
		}
	} else {
		if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
			return err
		}
	}
	cmdArgs := withScope(scope, []string{"daemon-reload"})
	cmdName := "systemctl"
	if useSudo {
		cmdName = "sudo"
		cmdArgs = append([]string{"systemctl"}, cmdArgs...)
	}
	cmd := exec.Command(cmdName, cmdArgs...)
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

func runSystemctl(scope Scope, useSudo bool, args ...string) error {
	cmdArgs := withScope(scope, args)
	cmdName := "systemctl"
	if useSudo {
		cmdName = "sudo"
		cmdArgs = append([]string{"systemctl"}, cmdArgs...)
	}
	cmd := exec.Command(cmdName, cmdArgs...)
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
