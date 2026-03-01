// Copyright 2017-2026 DERO Project. All rights reserved.

//go:build !windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func setupProgramOutput() (*os.File, func()) {
	origStdout, err := unix.Dup(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Printf("Warning: failed to dup stdout: %v\n", err)
		origStdout = -1
	}
	origStderr, err := unix.Dup(int(os.Stderr.Fd()))
	if err != nil {
		fmt.Printf("Warning: failed to dup stderr: %v\n", err)
		origStderr = -1
	}

	ttyOut, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		ttyOut = os.Stdout
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		_ = unix.Dup2(int(devNull.Fd()), int(os.Stdout.Fd()))
		_ = unix.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))
		_ = devNull.Close()
	}

	restored := false
	restore := func() {
		if restored {
			return
		}
		restored = true
		if origStdout >= 0 {
			_ = unix.Dup2(origStdout, int(os.Stdout.Fd()))
			_ = unix.Close(origStdout)
		}
		if origStderr >= 0 {
			_ = unix.Dup2(origStderr, int(os.Stderr.Fd()))
			_ = unix.Close(origStderr)
		}
		if ttyOut != nil && ttyOut != os.Stdout {
			_ = ttyOut.Close()
		}
	}

	return ttyOut, restore
}
