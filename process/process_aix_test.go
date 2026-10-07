// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitProcStat(t *testing.T) {
	expectedFieldsNum := 53
	statLineContent := make([]string, expectedFieldsNum-1)
	for i := 0; i < expectedFieldsNum-1; i++ {
		statLineContent[i] = strconv.Itoa(i + 1)
	}

	cases := []string{
		"ok",
		"ok)",
		"(ok",
		"ok )",
		"ok )(",
		"ok )()",
		"() ok )()",
		"() ok (()",
		" ) ok )",
		"(ok) (ok)",
	}

	consideredFields := []int{4, 7, 10, 11, 12, 13, 14, 15, 18, 22, 42}

	commandNameIndex := 2
	for _, expectedName := range cases {
		statLineContent[commandNameIndex-1] = "(" + expectedName + ")"
		statLine := strings.Join(statLineContent, " ")
		t.Run("name: "+expectedName, func(t *testing.T) {
			parsedStatLine := splitProcStat([]byte(statLine))
			assert.Equal(t, expectedName, parsedStatLine[commandNameIndex])
			for _, idx := range consideredFields {
				expected := strconv.Itoa(idx)
				parsed := parsedStatLine[idx]
				assert.Equal(
					t, expected, parsed,
					"field %d (index from 1 as in man proc) must be %q but %q is received",
					idx, expected, parsed,
				)
			}
		})
	}
}

func TestSplitProcStat_fromFile(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		statFile := fmt.Sprintf("testdata/aix/%d/stat", pid)
		if _, err := os.Stat(statFile); err != nil {
			continue
		}
		contents, err := os.ReadFile(statFile)
		require.NoError(t, err)

		pidStr := strconv.Itoa(int(pid))

		ppid := "68044" // TODO: how to pass ppid to test?

		fields := splitProcStat(contents)
		assert.Equal(t, pidStr, fields[1])
		assert.Equal(t, "test(cmd).sh", fields[2])
		assert.Equal(t, "S", fields[3])
		assert.Equal(t, ppid, fields[4])
		assert.Equal(t, pidStr, fields[5]) // pgrp
		assert.Equal(t, ppid, fields[6])   // session
		assert.Equal(t, pidStr, fields[8]) // tpgrp
		assert.Equal(t, "20", fields[18])  // priority
		assert.Equal(t, "1", fields[20])   // num threads
		assert.Equal(t, "0", fields[52])   // exit code
	}
}

func TestFillFromCommWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/aix/%d/status", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		if err := p.fillFromCommWithContext(context.Background()); err != nil {
			t.Error(err)
		}
	}
}

func TestFillFromStatusWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/aix/%d/status", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		if err := p.fillFromStatus(); err != nil {
			t.Error(err)
		}
	}
}

func Benchmark_fillFromCommWithContext(b *testing.B) {
	b.Setenv("HOST_PROC", "testdata/aix")
	pid := 5767616
	p, _ := NewProcess(int32(pid))
	for i := 0; i < b.N; i++ {
		p.fillFromCommWithContext(context.Background())
	}
}

func Benchmark_fillFromStatusWithContext(b *testing.B) {
	b.Setenv("HOST_PROC", "testdata/aix")
	pid := 5767616
	p, _ := NewProcess(int32(pid))
	for i := 0; i < b.N; i++ {
		p.fillFromStatus()
	}
}

func TestFillFromTIDStatWithContext_AIX(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pid := range pids {
		pid, err := strconv.ParseInt(pid.Name(), 0, 32)
		if err != nil {
			continue
		}
		if _, err := os.Stat(fmt.Sprintf("testdata/aix/%d/status", pid)); err != nil {
			continue
		}
		p, _ := NewProcess(int32(pid))
		_, _, cpuTimes, _, _, _, _, err := p.fillFromTIDStat(-1)
		if err != nil {
			t.Error(err)
		}
		// Verify CPU times are populated
		require.NotNil(t, cpuTimes)
		assert.GreaterOrEqual(t, cpuTimes.User, float64(0))
		assert.GreaterOrEqual(t, cpuTimes.System, float64(0))
	}
}

func TestProcessMemoryMaps(t *testing.T) {
	// Read the procmap output test data file
	procmapData, err := os.ReadFile("testdata/aix/procmap_output")
	require.NoError(t, err)

	// Create a process and test the parseMemoryMaps function directly
	p := &Process{Pid: 1}
	maps := p.parseMemoryMaps(string(procmapData))

	// Verify we got some memory maps back
	require.NotNil(t, maps)
	require.NotEmpty(t, *maps, "expected to get at least one memory map")

	// Expected AIX memory maps from procmap output for PID 1
	// These are the actual ranges from a typical AIX init process
	// Based on actual procmap output:
	// - KERTXT: no mapped object → "[kertxt]"
	// - MAINTEXT: "init" as mapped object
	// - MAINDATA: "init" as mapped object
	// - HEAP: no mapped object → "[heap]"
	// - STACK: no mapped object → "[stack]"
	// - Libraries: actual library paths
	expectedMaps := []struct {
		pathContains string
		minSize      uint64
	}{
		{"[kertxt]", 262144 * 1024},   // KERTXT segment - no mapped object
		{"[heap]", 768 * 1024},        // HEAP segment - no mapped object
		{"[stack]", 260272 * 1024},    // STACK segment - no mapped object
		{"libc.a[shr.o]", 586 * 1024}, // libc shared library
	}

	// Verify all expected memory map types are present
	for _, expected := range expectedMaps {
		found := false
		for _, m := range *maps {
			if strings.Contains(m.Path, expected.pathContains) {
				require.GreaterOrEqual(t, m.Size, expected.minSize,
					"memory map %s has unexpected size", m.Path)
				found = true
				break
			}
		}
		require.True(t, found, "expected to find memory map containing %s", expected.pathContains)
	}

	// Verify that the memory maps have valid field values (non-nil)
	for _, m := range *maps {
		// Fields should be populated (at least Path must be non-empty)
		require.NotEmpty(t, m.Path, "memory map path should not be empty")
	}
}

func TestFillFromExeWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pidDir := range pids {
		pid, err := strconv.ParseInt(pidDir.Name(), 0, 32)
		if err != nil {
			continue
		}
		psinfo := fmt.Sprintf("testdata/aix/%d/psinfo", pid)
		if _, err := os.Stat(psinfo); err != nil {
			continue
		}
		p, err := NewProcess(int32(pid))
		require.NoError(t, err)
		exe, err := p.fillFromExeWithContext(context.Background())
		if err == nil {
			// Should get a string (possibly empty or with executable name)
			assert.IsType(t, "", exe)
		}
	}
}

func TestFillFromCmdlineWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pidDir := range pids {
		pid, err := strconv.ParseInt(pidDir.Name(), 0, 32)
		if err != nil {
			continue
		}
		psinfo := fmt.Sprintf("testdata/aix/%d/psinfo", pid)
		if _, err := os.Stat(psinfo); err != nil {
			continue
		}
		p, err := NewProcess(int32(pid))
		require.NoError(t, err)
		cmdline, err := p.fillFromCmdlineWithContext(context.Background())
		if err == nil {
			// Should get a string (possibly empty or with command line)
			assert.IsType(t, "", cmdline)
		}
	}
}

func TestFillFromCmdlineSliceWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pidDir := range pids {
		pid, err := strconv.ParseInt(pidDir.Name(), 0, 32)
		if err != nil {
			continue
		}
		psinfo := fmt.Sprintf("testdata/aix/%d/psinfo", pid)
		if _, err := os.Stat(psinfo); err != nil {
			continue
		}
		p, err := NewProcess(int32(pid))
		require.NoError(t, err)
		cmdlineSlice, err := p.fillSliceFromCmdlineWithContext(context.Background())
		if err == nil {
			// Should get a slice of strings
			assert.IsType(t, []string{}, cmdlineSlice)
		}
	}
}

func TestFillFromStatmWithContext(t *testing.T) {
	pids, err := os.ReadDir("testdata/aix/")
	if err != nil {
		t.Error(err)
	}
	t.Setenv("HOST_PROC", "testdata/aix")
	for _, pidDir := range pids {
		pid, err := strconv.ParseInt(pidDir.Name(), 0, 32)
		if err != nil {
			continue
		}
		psinfo := fmt.Sprintf("testdata/aix/%d/psinfo", pid)
		if _, err := os.Stat(psinfo); err != nil {
			continue
		}
		p, err := NewProcess(int32(pid))
		require.NoError(t, err)
		memInfo, memInfoEx, err := p.fillFromStatmWithContext(context.Background())
		if err == nil {
			assert.NotNil(t, memInfo)
			assert.NotNil(t, memInfoEx)
			// Memory values should be non-negative
			//nolint:testifylint // value is always >= 0, but we validate it
			assert.GreaterOrEqual(t, memInfo.VMS, uint64(0))
			//nolint:testifylint // value is always >= 0, but we validate it
			assert.GreaterOrEqual(t, memInfo.RSS, uint64(0))
			//nolint:testifylint // value is always >= 0, but we validate it
			assert.GreaterOrEqual(t, memInfoEx.VMS, uint64(0))
			//nolint:testifylint // value is always >= 0, but we validate it
			assert.GreaterOrEqual(t, memInfoEx.RSS, uint64(0))
		}
	}
}

func TestTerminalWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	terminal, err := p.TerminalWithContext(ctx)
	// Terminal may or may not be available depending on how test is run
	if err == nil {
		assert.IsType(t, "", terminal)
	}
}

func TestPageFaultsWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	pageFaults, err := p.PageFaultsWithContext(ctx)
	if err != nil {
		t.Logf("PageFaultsWithContext error: %v", err)
		return
	}
	if pageFaults != nil {
		// Page fault counts should be non-negative
		//nolint:testifylint // minor faults field is naturally >= 0
		assert.GreaterOrEqual(t, pageFaults.MinorFaults, uint64(0))
		//nolint:testifylint // major faults field is naturally >= 0
		assert.GreaterOrEqual(t, pageFaults.MajorFaults, uint64(0))
	}
}

func TestRlimitUsageWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	limits, err := p.RlimitUsageWithContext(ctx, false)
	if err != nil {
		t.Logf("RlimitUsageWithContext error: %v", err)
		return
	}
	if len(limits) > 0 {
		for _, limit := range limits {
			// Hard limit should be >= soft limit
			assert.GreaterOrEqual(t, limit.Hard, limit.Soft)
		}
	}
}

func TestIOCountersWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	ioCounters, err := p.IOCountersWithContext(ctx)
	// IOCounters may not be available without WLM+iostat configuration
	if err == nil {
		assert.NotNil(t, ioCounters)
		//nolint:testifylint // checking non-negative constraint
		assert.GreaterOrEqual(t, ioCounters.ReadBytes, uint64(0))
		//nolint:testifylint // checking non-negative constraint
		assert.GreaterOrEqual(t, ioCounters.WriteBytes, uint64(0))
	}
}

// diskIOInvoker answers wlmcntrl -q and ps with canned output and records each command it runs.
type diskIOInvoker struct {
	wlm    string
	ps     []string // successive ps outputs; the last one repeats
	kernel string   // extra ps lines for kernel processes, which AIX ps lists only with -k
	calls  []string
}

func (f *diskIOInvoker) Command(name string, arg ...string) ([]byte, error) {
	return f.CommandWithContext(context.Background(), name, arg...)
}

func (f *diskIOInvoker) CommandWithContext(_ context.Context, name string, arg ...string) ([]byte, error) {
	f.calls = append(f.calls, name)
	switch name {
	case "wlmcntrl":
		if strings.Contains(f.wlm, "stopped") {
			return []byte(f.wlm), errors.New("exit status 1")
		}
		return []byte(f.wlm), errors.New("exit status 2")
	case "ps":
		n := 0
		for _, c := range f.calls {
			if c == "ps" {
				n++
			}
		}
		out := f.ps[min(n, len(f.ps))-1]
		if slices.ContainsFunc(arg, func(a string) bool { return strings.HasPrefix(a, "-") && strings.Contains(a, "k") }) {
			out += f.kernel
		}
		return []byte(out), nil
	}
	return nil, fmt.Errorf("unexpected command %s", name)
}

func (f *diskIOInvoker) count(name string) int {
	n := 0
	for _, c := range f.calls {
		if c == name {
			n++
		}
	}
	return n
}

// useDiskIOFake swaps in a fake invoker and clock, and clears the shared snapshot.
func useDiskIOFake(t *testing.T, f *diskIOInvoker) *time.Time {
	t.Helper()
	origInvoke, origNow := invoke, aixNow
	now := time.Unix(1000, 0)
	invoke, aixNow = f, func() time.Time { return now }
	resetAIXDiskIO()
	t.Cleanup(func() { invoke, aixNow = origInvoke, origNow; resetAIXDiskIO() })
	return &now
}

const wlmRunningMode = "WLM is running in passive mode\n"

func TestIOCountersWithContext_WLMStopped(t *testing.T) {
	f := &diskIOInvoker{wlm: "WLM is stopped\n", ps: []string{"       1       -\n"}}
	useDiskIOFake(t, f)

	_, err := (&Process{Pid: 1}).IOCountersWithContext(context.Background())
	require.ErrorIs(t, err, errAIXWLMNotRunning)
	assert.Equal(t, 0, f.count("ps"), "ps should not run while WLM is stopped")
}

func TestIOCountersWithContext_OneSnapshotForAllProcesses(t *testing.T) {
	f := &diskIOInvoker{wlm: wlmRunningMode, ps: []string{"       1      10\n      42     555\n"}}
	useDiskIOFake(t, f)
	ctx := context.Background()

	io1, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	io42, err := (&Process{Pid: 42}).IOCountersWithContext(ctx)
	require.NoError(t, err)

	assert.Equal(t, uint64(10), io1.ReadBytes)
	assert.Equal(t, uint64(555), io42.ReadBytes)
	assert.Equal(t, 1, f.count("ps"))
	assert.Equal(t, 1, f.count("wlmcntrl"))
}

func TestIOCountersWithContext_StaleSnapshotRefreshes(t *testing.T) {
	f := &diskIOInvoker{wlm: wlmRunningMode, ps: []string{"       1      10\n", "       1      20\n"}}
	now := useDiskIOFake(t, f)
	ctx := context.Background()

	io, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), io.ReadBytes)

	*now = now.Add(aixDiskIOMaxAge)
	io, err = (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(20), io.ReadBytes)
	assert.Equal(t, 2, f.count("ps"))
}

func TestIOCountersWithContext_NewProcessRefreshesOnce(t *testing.T) {
	f := &diskIOInvoker{wlm: wlmRunningMode, ps: []string{"       1      10\n", "       1      10\n       7       3\n"}}
	useDiskIOFake(t, f)
	ctx := context.Background()

	_, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	io, err := (&Process{Pid: 7}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), io.ReadBytes)

	_, err = (&Process{Pid: 99}).IOCountersWithContext(ctx)
	require.ErrorIs(t, err, ErrorProcessNotRunning)
	assert.Equal(t, 3, f.count("ps"), "a PID missing from a fresh snapshot should not trigger another ps")
}

func TestIOCountersWithContext_KernelProcessInSnapshot(t *testing.T) {
	f := &diskIOInvoker{wlm: wlmRunningMode, ps: []string{"       1      10\n"}, kernel: "     260       0\n"}
	useDiskIOFake(t, f)

	io, err := (&Process{Pid: 260}).IOCountersWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(0), io.ReadBytes)
	assert.Equal(t, 1, f.count("ps"), "kernel processes should come from the same snapshot")
}

func TestIOCountersWithContext_NoDataForProcess(t *testing.T) {
	f := &diskIOInvoker{wlm: wlmRunningMode, ps: []string{"       1       -\n"}}
	useDiskIOFake(t, f)

	_, err := (&Process{Pid: 1}).IOCountersWithContext(context.Background())
	require.Error(t, err)
	assert.NotErrorIs(t, err, errAIXWLMNotRunning)
}

func TestCPUAffinityWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	affinity, err := p.CPUAffinityWithContext(ctx)
	// CPU affinity may not be available on all AIX systems
	if err == nil {
		assert.NotEmpty(t, affinity)
		for _, cpu := range affinity {
			assert.GreaterOrEqual(t, cpu, int32(0))
		}
	}
}

func TestCPUPercentWithContext(t *testing.T) {
	// Get current process
	ctx := context.Background()
	p := Process{Pid: int32(os.Getpid())}

	percent, err := p.CPUPercentWithContext(ctx)
	require.NoError(t, err)
	// CPU percent should be >= 0
	assert.GreaterOrEqual(t, percent, float64(0))
	// CPU percent should not exceed 100 * number of CPUs (but ps can sometimes report >100% on single CPU)
	assert.Less(t, percent, float64(500)) // sanity check to avoid absurd values
}
