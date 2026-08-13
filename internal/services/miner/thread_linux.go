// Copyright 2017-2026 DERO Project. All rights reserved.

package miner

import (
	"runtime"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

var processor int32

func threadaffinity() {
	var cpuset unix.CPUSet

	lockOnCPU := atomic.AddInt32(&processor, 1)
	if lockOnCPU >= int32(runtime.GOMAXPROCS(0)) {
		return
	}
	cpuset.Zero()
	cpuset.Set(int(avoidHT(int(lockOnCPU))))

	unix.SchedSetaffinity(0, &cpuset)
}

func avoidHT(i int) int {
	count := runtime.GOMAXPROCS(0)
	if i < count/2 {
		return i * 2
	}
	return (i-count/2)*2 + 1
}
