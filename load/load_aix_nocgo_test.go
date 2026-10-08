// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package load

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// mockInvoker returns canned output for specific commands.
type mockInvoker struct {
	common.Invoke
	output map[string][]byte
}

func (m mockInvoker) CommandWithContext(_ context.Context, name string, arg ...string) ([]byte, error) {
	var b strings.Builder
	b.WriteString(name)
	for _, a := range arg {
		b.WriteString(" ")
		b.WriteString(a)
	}
	key := b.String()
	if out, ok := m.output[key]; ok {
		return out, nil
	}
	return nil, common.ErrNotImplementedError
}

func TestMiscWithContext_FromThreadStates(t *testing.T) {
	origProcs, origThreads := aixProcesses, aixThreads
	t.Cleanup(func() { aixProcesses, aixThreads, testInvoker = origProcs, origThreads, nil })
	testInvoker = mockInvoker{} // no vmstat output; Misc leaves those counters at zero
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
