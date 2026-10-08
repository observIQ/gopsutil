// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package load

import (
	"context"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstat and libc reads, swapped out in tests.
var (
	perfstatCPUTotalStat = common.PerfstatCPUTotalStat
	aixProcesses         = common.AIXProcesses
	aixThreads           = common.AIXThreads
)

func AvgWithContext(_ context.Context) (*AvgStat, error) {
	t, err := perfstatCPUTotalStat()
	if err != nil {
		return nil, err
	}
	load := func(v uint64) float64 { return float64(v) / common.AIXLoadAvgScale }
	return &AvgStat{Load1: load(t.LoadAvg[0]), Load5: load(t.LoadAvg[1]), Load15: load(t.LoadAvg[2])}, nil
}

// MiscWithContext counts processes from getprocs64 and getthrds64, leaving out kernel
// processes. A process is running when one of its threads is runnable, and blocked when
// one waits in the VMM wait queue, the same queues vmstat reports as r and b.
func MiscWithContext(_ context.Context) (*MiscStat, error) {
	procs, err := aixProcesses()
	if err != nil {
		return nil, err
	}
	threads, err := aixThreads()
	if err != nil {
		return nil, err
	}

	ret := &MiscStat{}
	for i := range procs {
		if procs[i].Flags&common.AIXProcKernel == 0 {
			ret.ProcsTotal++
		}
	}
	running, blocked := map[int32]bool{}, map[int32]bool{}
	for i := range threads {
		th := &threads[i]
		switch {
		case th.Flags&common.AIXThreadKernel != 0:
		case th.State == common.AIXThreadRunnable:
			running[th.Pid] = true
		case th.State == common.AIXThreadSleeping && common.AIXVMMWait(th.WaitType):
			blocked[th.Pid] = true
		}
	}
	ret.ProcsRunning, ret.ProcsBlocked = len(running), len(blocked)

	// Cumulative since-boot counters.
	t, err := perfstatCPUTotalStat()
	if err != nil {
		return nil, err
	}
	ret.Ctxt, ret.SysCalls, ret.Interrupts = int(t.PSwitch), int(t.Syscall), int(t.DevIntrs)

	return ret, nil
}

// getVmstatMetrics runs vmstat -s and returns cumulative since-boot counters
// for context switches, device interrupts, and syscalls.
