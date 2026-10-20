// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package cpu

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// testInvoker is a mock invoker that reads fixture files from testdata/aix/.
// File naming follows FakeInvoke convention: command name + args concatenated.
type testInvoker struct {
	common.FakeInvoke
}

func (testInvoker) Command(name string, arg ...string) ([]byte, error) {
	var b strings.Builder
	b.WriteString(filepath.Base(name))
	for _, a := range arg {
		b.WriteString(a)
	}
	return os.ReadFile(filepath.Join("testdata", "aix", b.String()))
}

func (t testInvoker) CommandWithContext(_ context.Context, name string, arg ...string) ([]byte, error) {
	return t.Command(name, arg...)
}

func perfstatName(s string) (b [64]byte) {
	copy(b[:], s)
	return b
}

func useFakePerfstat(t *testing.T, cpus []common.PerfstatCPU, total *common.PerfstatCPUTotal) {
	t.Helper()
	origCPUs, origTotal := perfstatCPUStats, perfstatCPUTotalStat
	perfstatCPUStats = func() ([]common.PerfstatCPU, error) { return cpus, nil }
	perfstatCPUTotalStat = func() (*common.PerfstatCPUTotal, error) { return total, nil }
	t.Cleanup(func() { perfstatCPUStats, perfstatCPUTotalStat = origCPUs, origTotal })
}

func TestTimesWithContext_PerCPUFromPerfstat(t *testing.T) {
	useFakePerfstat(t, []common.PerfstatCPU{
		{Name: perfstatName("cpu0"), User: 250, Sys: 150, Idle: 9500, Wait: 100},
		{Name: perfstatName("cpu1"), User: 0, Sys: 30, Idle: 9970, Wait: 0},
	}, nil)

	stats, err := TimesWithContext(context.Background(), true)
	require.NoError(t, err)
	// perfstat counts 100 ticks per second.
	assert.Equal(t, []TimesStat{
		{CPU: "cpu0", User: 2.5, System: 1.5, Idle: 95, Iowait: 1},
		{CPU: "cpu1", User: 0, System: 0.3, Idle: 99.7, Iowait: 0},
	}, stats)
}

func TestTimesWithContext_TotalFromPerfstat(t *testing.T) {
	useFakePerfstat(t, nil, &common.PerfstatCPUTotal{User: 257821, Sys: 197820, Idle: 679600884, Wait: 3855})

	stats, err := TimesWithContext(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, []TimesStat{
		{CPU: "cpu-total", User: 2578.21, System: 1978.2, Idle: 6796008.84, Iowait: 38.55},
	}, stats)
}

func TestInfoWithContext(t *testing.T) {
	origInvoke := invoke
	invoke = testInvoker{}
	// InfoWithContext caches prtconf output via prtconfOnce; reset it so the
	// fixture invoker is used even if a prior call already populated the cache.
	prtconfOnce = sync.Once{}
	defer func() {
		invoke = origInvoke
		prtconfOnce = sync.Once{}
	}()

	info, err := InfoWithContext(context.Background())
	require.NoError(t, err)
	require.Len(t, info, 1)

	c := info[0]
	assert.Equal(t, "IBM pSeries (emulated by qemu)", c.VendorID)
	assert.Equal(t, "PowerPC", c.Family)
	assert.Equal(t, "POWER8", c.Model)
	assert.Equal(t, "PowerPC_POWER8", c.ModelName)
	assert.Equal(t, int32(4), c.Cores)
	assert.InDelta(t, 1000.0, c.Mhz, 0.01)
}

func TestCountsWithContext(t *testing.T) {
	origInvoke := invoke
	invoke = testInvoker{}
	prtconfOnce = sync.Once{}
	defer func() {
		invoke = origInvoke
		prtconfOnce = sync.Once{}
	}()
	// prtconf reports 4 virtual processors; with SMT8 that is 32 logical CPUs.
	useFakePerfstat(t, nil, &common.PerfstatCPUTotal{NCPUs: 32, NCPUsCfg: 32})

	logical, err := CountsWithContext(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, 32, logical)

	physical, err := CountsWithContext(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, 4, physical)
}
