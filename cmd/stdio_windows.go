// Copyright 2017-2026 DERO Project. All rights reserved.

//go:build windows

package main

import "os"

func setupProgramOutput() (*os.File, func()) {
	return os.Stdout, func() {}
}
