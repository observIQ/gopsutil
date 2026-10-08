// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package mem

import (
	"context"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstat reads, swapped out in tests.
var (
	perfstatMemoryTotalStat  = common.PerfstatMemoryTotalStat
	perfstatPagingSpaceStats = common.PerfstatPagingSpaceStats
)

// perfstat counts memory in 4 KB pages.
const perfstatPage = 4096

func VirtualMemoryWithContext(_ context.Context) (*VirtualMemoryStat, error) {
	m, err := perfstatMemoryTotalStat()
	if err != nil {
		return nil, err
	}
	if m.RealTotal == 0 {
		return nil, common.ErrNotImplementedError
	}
	return &VirtualMemoryStat{
		Total:       m.RealTotal * perfstatPage,
		Available:   m.RealAvail * perfstatPage,
		Free:        m.RealFree * perfstatPage,
		Used:        m.RealInUse * perfstatPage,
		UsedPercent: 100 * float64(m.RealInUse) / float64(m.RealTotal),
		Active:      m.VirtActive * perfstatPage,
		SwapTotal:   m.PgSpTotal * perfstatPage,
		SwapFree:    m.PgSpFree * perfstatPage,
	}, nil
}

// SwapMemoryWithContext reports paging-space usage. Sin, Sout, PgIn and PgOut stay page
// counts, since the hostmetrics paging scraper records them as operations.
func SwapMemoryWithContext(_ context.Context) (*SwapMemoryStat, error) {
	m, err := perfstatMemoryTotalStat()
	if err != nil {
		return nil, err
	}
	if m.PgSpTotal == 0 {
		return nil, common.ErrNotImplementedError
	}
	used := m.PgSpTotal - m.PgSpFree
	return &SwapMemoryStat{
		Total:       m.PgSpTotal * perfstatPage,
		Free:        m.PgSpFree * perfstatPage,
		Used:        used * perfstatPage,
		UsedPercent: 100 * float64(used) / float64(m.PgSpTotal),
		Sin:         m.PgSpIns,
		Sout:        m.PgSpOuts,
		PgIn:        m.PgIns,
		PgOut:       m.PgOuts,
	}, nil
}

func SwapDevicesWithContext(_ context.Context) ([]*SwapDevice, error) {
	spaces, err := perfstatPagingSpaceStats()
	if err != nil {
		return nil, err
	}
	ret := make([]*SwapDevice, 0, len(spaces))
	for i := range spaces {
		sp := &spaces[i]
		ret = append(ret, &SwapDevice{
			Name:      common.CString(sp.Name[:]),
			UsedBytes: uint64(sp.MBUsed) << 20,
			FreeBytes: uint64(sp.MBSize-sp.MBUsed) << 20,
		})
	}
	return ret, nil
}
