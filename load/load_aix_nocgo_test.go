// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package load

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func TestMiscWithContext_FromThreadStates(t *testing.T) {
	origProcs, origThreads := aixProcesses, aixThreads
	t.Cleanup(func() { aixProcesses, aixThreads = origProcs, origThreads })
	aixProcesses = func() ([]common.AIXProcess, error) {
		return []common.AIXProcess{
			{Pid: 1},
			{Pid: 260, Flags: common.AIXProcKernel},
			{Pid: 300},
			{Pid: 400},
			{Pid: 500}, // a zombie, with no threads left
		}, nil
	}
	aixThreads = func() ([]common.AIXThread, error) {
		return []common.AIXThread{
			{Pid: 260, State: common.AIXThreadRunnable, Flags: common.AIXThreadKernel}, // kernel wait process
			{Pid: 1, State: common.AIXThreadSleeping, WaitType: 1},                     // TWEVENT
			{Pid: 300, State: common.AIXThreadRunnable},
			{Pid: 300, State: common.AIXThreadRunnable},              // a second runnable thread, same process
			{Pid: 400, State: common.AIXThreadSleeping, WaitType: 5}, // TWPGIN
			{Pid: 400, State: common.AIXThreadSleeping, WaitType: 3}, // TWTIMER
		}, nil
	}

	misc, err := MiscWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, misc.ProcsRunning, "process 300; the kernel wait process is left out")
	assert.Equal(t, 1, misc.ProcsBlocked, "process 400, waiting on page-in")
	assert.Equal(t, 4, misc.ProcsTotal, "every non-kernel process, the zombie included")
}

func TestAvgWithContext_FromPerfstat(t *testing.T) {
	orig := perfstatCPUTotalStat
	perfstatCPUTotalStat = func() (*common.PerfstatCPUTotal, error) {
		// perfstat load averages are fixed point, scaled by 1<<16.
		return &common.PerfstatCPUTotal{LoadAvg: [3]uint64{65536, 32768, 98304}}, nil
	}
	defer func() { perfstatCPUTotalStat = orig }()

	avg, err := AvgWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &AvgStat{Load1: 1, Load5: 0.5, Load15: 1.5}, avg)
}

func TestMiscWithContext_CountersFromPerfstat(t *testing.T) {
	origProcs, origThreads, origTotal := aixProcesses, aixThreads, perfstatCPUTotalStat
	t.Cleanup(func() { aixProcesses, aixThreads, perfstatCPUTotalStat = origProcs, origThreads, origTotal })
	aixProcesses = func() ([]common.AIXProcess, error) { return nil, nil }
	aixThreads = func() ([]common.AIXThread, error) { return nil, nil }
	perfstatCPUTotalStat = func() (*common.PerfstatCPUTotal, error) {
		return &common.PerfstatCPUTotal{PSwitch: 134399557, Syscall: 111916845, DevIntrs: 5057134}, nil
	}

	misc, err := MiscWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 134399557, misc.Ctxt)
	assert.Equal(t, 111916845, misc.SysCalls)
	assert.Equal(t, 5057134, misc.Interrupts)
}
