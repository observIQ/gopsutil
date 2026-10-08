// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package load

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstat and libc reads, swapped out in tests.
var (
	perfstatCPUTotalStat = common.PerfstatCPUTotalStat
	aixProcesses         = common.AIXProcesses
	aixThreads           = common.AIXThreads
)

// testInvoker is used for dependency injection in tests
var testInvoker common.Invoker

// getInvoker returns the test invoker if set, otherwise returns the default
func getInvoker() common.Invoker {
	if testInvoker != nil {
		return testInvoker
	}
	return common.Invoke{}
}

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
func MiscWithContext(ctx context.Context) (*MiscStat, error) {
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

	// Get context switches, interrupts, and syscalls from vmstat.
	// These are cumulative since-boot counters from vmstat -s.
	ctxt, interrupts, syscalls, err := getVmstatMetrics(ctx)
	if err == nil {
		ret.Ctxt = ctxt
		ret.Interrupts = interrupts
		ret.SysCalls = syscalls
	}

	return ret, nil
}

// getVmstatMetrics runs vmstat -s and returns cumulative since-boot counters
// for context switches, device interrupts, and syscalls.
func getVmstatMetrics(ctx context.Context) (ctxt, interrupts, syscalls int, err error) {
	out, err := getInvoker().CommandWithContext(ctx, "vmstat", "-s")
	if err != nil {
		return 0, 0, 0, err
	}

	// vmstat -s output format: <whitespace><number> <description>
	// Example lines:
	//   5842393706 cpu context switches
	//      33412179 device interrupts
	//  12918944607 syscalls
	lines := strings.Split(string(out), "\n")
	foundCtxt, foundInterrupts, foundSyscalls := false, false, false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split into number and description at the first space after the number
		idx := strings.IndexByte(line, ' ')
		if idx < 0 {
			continue
		}
		valStr := line[:idx]
		desc := strings.TrimSpace(line[idx+1:])

		v, parseErr := strconv.Atoi(valStr)
		if parseErr != nil {
			continue
		}

		switch desc {
		case "cpu context switches":
			ctxt = v
			foundCtxt = true
		case "device interrupts":
			interrupts = v
			foundInterrupts = true
		case "syscalls":
			syscalls = v
			foundSyscalls = true
		}
	}

	if !foundCtxt || !foundInterrupts || !foundSyscalls {
		return ctxt, interrupts, syscalls, errors.New("incomplete vmstat output: missing expected counters")
	}
	return ctxt, interrupts, syscalls, nil
}
