// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package cpu

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstat reads, swapped out in tests.
var (
	perfstatCPUStats     = common.PerfstatCPUStats
	perfstatCPUTotalStat = common.PerfstatCPUTotalStat
)

// prtconfOutput caches the output of prtconf, which returns static CPU
// hardware information (model, speed, count) that does not change at runtime.
// Caching avoids spawning a subprocess on every InfoWithContext call.
var (
	prtconfOnce   sync.Once
	prtconfOutput []byte
	prtconfErr    error
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

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	prtconfOnce.Do(func() {
		prtconfOutput, prtconfErr = invoke.CommandWithContext(ctx, "prtconf")
	})
	if prtconfErr != nil {
		return nil, prtconfErr
	}
	out := prtconfOutput

	ret := InfoStat{}
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "Number Of Processors:"):
			p := strings.Fields(line)
			if len(p) > 3 {
				if t, err := strconv.ParseUint(p[3], 10, 64); err == nil {
					ret.Cores = int32(t)
				}
			}
		case strings.HasPrefix(line, "Processor Clock Speed:"):
			p := strings.Fields(line)
			if len(p) > 4 {
				if t, err := strconv.ParseFloat(p[3], 64); err == nil {
					switch strings.ToUpper(p[4]) {
					case "MHZ":
						ret.Mhz = t
					case "GHZ":
						ret.Mhz = t * 1000.0
					case "KHZ":
						ret.Mhz = t / 1000.0
					default:
						ret.Mhz = t
					}
				}
			}
		case strings.HasPrefix(line, "System Model:"):
			p := strings.Split(string(line), ":")
			if p != nil {
				ret.VendorID = strings.TrimSpace(p[1])
			}
		case strings.HasPrefix(line, "Processor Type:"):
			p := strings.Split(string(line), ":")
			if p != nil {
				ret.ModelName = strings.TrimSpace(p[1])
				c := strings.Split(string(p[1]), "_")
				if c != nil {
					ret.Family = strings.TrimSpace(c[0])
					ret.Model = strings.TrimSpace(c[1])
				}
			}
		}
	}
	return []InfoStat{ret}, nil
}

func CountsWithContext(ctx context.Context, _ bool) (int, error) {
	info, err := InfoWithContext(ctx)
	if err == nil {
		return int(info[0].Cores), nil
	}
	return 0, err
}
