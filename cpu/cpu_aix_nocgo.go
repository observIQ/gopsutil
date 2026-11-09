// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package cpu

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstat reads, swapped out in tests.
var (
	perfstatCPUStats           = common.PerfstatCPUStats
	perfstatCPUTotalStat       = common.PerfstatCPUTotalStat
	perfstatPartitionTotalStat = common.PerfstatPartitionTotalStat
)

// TimesWithContext returns cumulative CPU time in seconds from perfstat tick counters.
func TimesWithContext(_ context.Context, percpu bool) ([]TimesStat, error) {
	seconds := func(ticks uint64) float64 { return float64(ticks) / common.AIXClockTicks }
	if !percpu {
		t, err := perfstatCPUTotalStat()
		if err != nil {
			return nil, err
		}
		return []TimesStat{{CPU: "cpu-total", User: seconds(t.User), System: seconds(t.Sys), Idle: seconds(t.Idle), Iowait: seconds(t.Wait)}}, nil
	}
	cpus, err := perfstatCPUStats()
	if err != nil {
		return nil, err
	}
	ret := make([]TimesStat, 0, len(cpus))
	for i := range cpus {
		c := &cpus[i]
		ret = append(ret, TimesStat{CPU: common.CString(c.Name[:]), User: seconds(c.User), System: seconds(c.Sys), Idle: seconds(c.Idle), Iowait: seconds(c.Wait)})
	}
	return ret, nil
}

// InfoWithContext reads CPU details from perfstat rather than prtconf, which takes
// seconds on AIX and outlasts short caller timeouts.
func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	total, err := perfstatCPUTotalStat()
	if err != nil {
		return nil, err
	}
	partition, err := perfstatPartitionTotalStat()
	if err != nil {
		return nil, err
	}
	model, err := invoke.CommandWithContext(ctx, "uname", "-M")
	if err != nil {
		return nil, err
	}

	ret := InfoStat{
		VendorID:  strings.TrimSpace(string(model)),
		ModelName: common.CString(total.Description[:]),
		Cores:     partition.OnlineCPUs,
		Mhz:       float64(total.ProcessorHz) / 1e6,
	}
	if family, cpuModel, ok := strings.Cut(ret.ModelName, "_"); ok {
		ret.Family, ret.Model = family, cpuModel
	}
	return []InfoStat{ret}, nil
}

// CountsWithContext returns the configured logical CPUs from perfstat, as the cgo
// build does, or the online virtual processors for the physical count.
func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	if logical {
		t, err := perfstatCPUTotalStat()
		if err != nil {
			return 0, err
		}
		return int(t.NCPUsCfg), nil
	}
	info, err := InfoWithContext(ctx)
	if err == nil {
		return int(info[0].Cores), nil
	}
	return 0, err
}
