// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package mem

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

const page = 4096 // perfstat counts memory in 4 KB pages

func useFakeMemory(t *testing.T) {
	t.Helper()
	origMem, origSpaces := perfstatMemoryTotalStat, perfstatPagingSpaceStats
	perfstatMemoryTotalStat = func() (*common.PerfstatMemoryTotal, error) {
		return &common.PerfstatMemoryTotal{
			RealTotal: 1048576, RealFree: 466719, RealInUse: 581857, RealAvail: 543420, VirtActive: 463068,
			PgSpTotal: 2621440, PgSpFree: 2614864, PgSpIns: 132, PgSpOuts: 3026, PgIns: 5000, PgOuts: 7000,
		}, nil
	}
	perfstatPagingSpaceStats = func() ([]common.PerfstatPagingSpace, error) {
		var hd6 common.PerfstatPagingSpace
		copy(hd6.Name[:], "hd6")
		hd6.MBSize, hd6.MBUsed = 10240, 10
		return []common.PerfstatPagingSpace{hd6}, nil
	}
	t.Cleanup(func() { perfstatMemoryTotalStat, perfstatPagingSpaceStats = origMem, origSpaces })
}

func TestVirtualMemoryWithContext_FromPerfstat(t *testing.T) {
	useFakeMemory(t)

	v, err := VirtualMemoryWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(1048576*page), v.Total)
	assert.Equal(t, uint64(543420*page), v.Available)
	assert.Equal(t, uint64(466719*page), v.Free)
	assert.Equal(t, uint64(581857*page), v.Used)
	assert.InDelta(t, 100*581857/1048576.0, v.UsedPercent, 1e-9)
	assert.Equal(t, uint64(463068*page), v.Active)
	assert.Equal(t, uint64(2621440*page), v.SwapTotal)
	assert.Equal(t, uint64(2614864*page), v.SwapFree)
}

func TestSwapMemoryWithContext_FromPerfstat(t *testing.T) {
	useFakeMemory(t)

	s, err := SwapMemoryWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(2621440*page), s.Total)
	assert.Equal(t, uint64(2614864*page), s.Free)
	assert.Equal(t, uint64((2621440-2614864)*page), s.Used)
	assert.InDelta(t, 100*(2621440-2614864)/2621440.0, s.UsedPercent, 1e-9)
	// The paging scraper records these as operation counts, so they stay page counts.
	assert.Equal(t, uint64(132), s.Sin)
	assert.Equal(t, uint64(3026), s.Sout)
	assert.Equal(t, uint64(5000), s.PgIn)
	assert.Equal(t, uint64(7000), s.PgOut)
}

func TestSwapDevicesWithContext_FromPerfstat(t *testing.T) {
	useFakeMemory(t)

	devs, err := SwapDevicesWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []*SwapDevice{{Name: "hd6", UsedBytes: 10 << 20, FreeBytes: 10230 << 20}}, devs)
}
