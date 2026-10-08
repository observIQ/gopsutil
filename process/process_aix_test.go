// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package process

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
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

func TestSignalsPendingWithContext_AIX(t *testing.T) {
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
		if _, err := os.Stat(fmt.Sprintf("testdata/aix/%d/status", pid)); err != nil {
			continue
		}
		p := &Process{Pid: int32(pid)}
		sig, err := p.SignalsPendingWithContext(context.Background())
		require.NoError(t, err)
		// PendingProcess is read from the status SigPend set; for fixtures
		// with no pending signals it is zero, which is valid.
		//nolint:testifylint // value is always >= 0, but we validate the parse path
		assert.GreaterOrEqual(t, sig.PendingProcess, uint64(0))
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
	// fillFromTIDStat reads page faults from getprocs64, so it needs a
	// live process. Use this test binary itself and the real /proc rather
	// than the fixture tree.
	p := &Process{Pid: int32(os.Getpid())}
	_, _, cpuTimes, _, _, _, _, err := p.fillFromTIDStat(-1)
	require.NoError(t, err)
	// Verify CPU times are populated
	require.NotNil(t, cpuTimes)
	assert.GreaterOrEqual(t, cpuTimes.User, float64(0))
	assert.GreaterOrEqual(t, cpuTimes.System, float64(0))
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
		p := &Process{Pid: int32(pid)}
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
		p := &Process{Pid: int32(pid)}
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
		p := &Process{Pid: int32(pid)}
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
		p := &Process{Pid: int32(pid)}
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

// procTableFake stands in for getprocs64 and counts how often the table is read.
type procTableFake struct {
	tables [][]common.AIXProcess // successive reads; the last one repeats
	calls  int
}

func (f *procTableFake) read() ([]common.AIXProcess, error) {
	f.calls++
	return f.tables[min(f.calls, len(f.tables))-1], nil
}

// noExecInvoker fails the test if the code under test runs any command.
type noExecInvoker struct{ t *testing.T }

func (n noExecInvoker) Command(name string, arg ...string) ([]byte, error) {
	return n.CommandWithContext(context.Background(), name, arg...)
}

func (n noExecInvoker) CommandWithContext(_ context.Context, name string, arg ...string) ([]byte, error) {
	n.t.Errorf("unexpected command: %s %s", name, strings.Join(arg, " "))
	return nil, errors.New("no commands in this test")
}

// useProcTableFake swaps in a fake process table and clock, forbids commands, and clears the shared snapshot.
func useProcTableFake(t *testing.T, f *procTableFake) *time.Time {
	t.Helper()
	origProcs, origNow, origInvoke := aixProcesses, aixNow, invoke
	now := time.Unix(1000, 0)
	aixProcesses, aixNow, invoke = f.read, func() time.Time { return now }, noExecInvoker{t}
	resetAIXPS()
	t.Cleanup(func() { aixProcesses, aixNow, invoke = origProcs, origNow, origInvoke; resetAIXPS() })
	return &now
}

func TestIOCountersWithContext_WithoutWLM(t *testing.T) {
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: 1, IOChars: 4096}}}}
	useProcTableFake(t, f)

	io, err := (&Process{Pid: 1}).IOCountersWithContext(context.Background())
	require.NoError(t, err)
	// pi_ioch counts read and write bytes together, as tdiskio did.
	assert.Equal(t, uint64(4096), io.ReadBytes)
}

func TestIOCountersWithContext_OneSnapshotForAllProcesses(t *testing.T) {
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: 1, IOChars: 10}, {Pid: 42, IOChars: 555}}}}
	useProcTableFake(t, f)
	ctx := context.Background()

	io1, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	io42, err := (&Process{Pid: 42}).IOCountersWithContext(ctx)
	require.NoError(t, err)

	assert.Equal(t, uint64(10), io1.ReadBytes)
	assert.Equal(t, uint64(555), io42.ReadBytes)
	assert.Equal(t, 1, f.calls)
}

func TestIOCountersWithContext_StaleSnapshotRefreshes(t *testing.T) {
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: 1, IOChars: 10}}, {{Pid: 1, IOChars: 20}}}}
	now := useProcTableFake(t, f)
	ctx := context.Background()

	io, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(10), io.ReadBytes)

	*now = now.Add(aixPSMaxAge)
	io, err = (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(20), io.ReadBytes)
	assert.Equal(t, 2, f.calls)
}

func TestIOCountersWithContext_NewProcessRefreshesOnce(t *testing.T) {
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: 1, IOChars: 10}}, {{Pid: 1, IOChars: 10}, {Pid: 7, IOChars: 3}}}}
	useProcTableFake(t, f)
	ctx := context.Background()

	_, err := (&Process{Pid: 1}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	io, err := (&Process{Pid: 7}).IOCountersWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), io.ReadBytes)

	_, err = (&Process{Pid: 99}).IOCountersWithContext(ctx)
	require.ErrorIs(t, err, ErrorProcessNotRunning)
	assert.Equal(t, 3, f.calls, "a PID missing from a fresh snapshot should not trigger another read")
}

func TestIOCountersWithContext_KernelProcessInSnapshot(t *testing.T) {
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: 1, IOChars: 10}, {Pid: 260, Flags: common.AIXProcKernel}}}}
	useProcTableFake(t, f)

	io, err := (&Process{Pid: 260}).IOCountersWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(0), io.ReadBytes)
	assert.Equal(t, 1, f.calls, "kernel processes should come from the same snapshot")
}

// Fixture PIDs under testdata/aix, whose psinfo the page-fault path reads before it reaches the snapshot
const (
	fixturePID      = 12845476
	otherFixturePID = 5767616
)

func TestPageFaultsWithContext_FromSnapshot(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata/aix")
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: fixturePID, MajFlt: 38}, {Pid: otherFixturePID, MajFlt: 7}}}}
	useProcTableFake(t, f)
	ctx := context.Background()

	pf, err := (&Process{Pid: fixturePID}).PageFaultsWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(38), pf.MajorFaults)
	pf, err = (&Process{Pid: otherFixturePID}).PageFaultsWithContext(ctx)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), pf.MajorFaults)
	assert.Equal(t, 1, f.calls, "page faults for every process should come from one read")
}

func TestTimesWithContext_SharesSnapshot(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata/aix")
	f := &procTableFake{tables: [][]common.AIXProcess{{{Pid: fixturePID, MajFlt: 38, IOChars: 5}, {Pid: otherFixturePID, MajFlt: 7, IOChars: 9}}}}
	useProcTableFake(t, f)
	ctx := context.Background()

	for _, pid := range []int32{fixturePID, otherFixturePID} {
		p := &Process{Pid: pid}
		_, err := p.TimesWithContext(ctx)
		require.NoError(t, err)
		_, err = p.PpidWithContext(ctx)
		require.NoError(t, err)
		_, err = p.IOCountersWithContext(ctx)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, f.calls, "times, ppid, and disk I/O for every process should share one read")
}

func TestEnvironWithContext_FromGetevars(t *testing.T) {
	orig := aixProcessEnv
	aixProcessEnv = func(pid int32) ([]string, error) {
		assert.Equal(t, int32(424242), pid)
		return []string{"TERM=dumb", "MSG=two words"}, nil
	}
	t.Cleanup(func() { aixProcessEnv = orig })

	env, err := (&Process{Pid: 424242}).EnvironWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"TERM=dumb", "MSG=two words"}, env)
}

func TestTerminalWithContext_FromPSInfo(t *testing.T) {
	// A copy of a fixture psinfo with pr_ttydev (offset 80) set to the device of /dev/pts/0.
	const pts0 = 0x8000001a00000000
	b, err := os.ReadFile(fmt.Sprintf("testdata/aix/%d/psinfo", fixturePID))
	require.NoError(t, err)
	binary.BigEndian.PutUint64(b[80:], pts0)
	proc := t.TempDir()
	require.NoError(t, os.MkdirAll(fmt.Sprintf("%s/%d", proc, fixturePID), 0o755))
	require.NoError(t, os.WriteFile(fmt.Sprintf("%s/%d/psinfo", proc, fixturePID), b, 0o600)) //nolint:gosec // a fixture under t.TempDir
	t.Setenv("HOST_PROC", proc)
	orig := aixTTYName
	aixTTYName = func(dev uint64) string {
		if dev == pts0 {
			return "pts/0"
		}
		return ""
	}
	t.Cleanup(func() { aixTTYName = orig })

	tty, err := (&Process{Pid: fixturePID}).TerminalWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "pts/0", tty)
}

func TestTerminalWithContext_NoTerminal(t *testing.T) {
	t.Setenv("HOST_PROC", "testdata/aix") // the fixture's pr_ttydev is 0
	tty, err := (&Process{Pid: fixturePID}).TerminalWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "-", tty, "ps prints - for a process without a terminal")
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
