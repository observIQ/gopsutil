// SPDX-License-Identifier: BSD-3-Clause

// Command gopsutil-sample exercises the cross-platform gopsutil API and prints
// each call's result, or its error, so the library's behavior on a given
// platform can be inspected at a glance. Every call is isolated: an error
// (including ErrNotImplementedError) or a panic is reported in place and the
// program continues, so it always runs to completion on any GOOS.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/nfs"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

func main() {
	ctx := context.Background()
	fmt.Printf("gopsutil sample — %s/%s (go %s)\n\n", runtime.GOOS, runtime.GOARCH, runtime.Version())

	// host
	report("host.Info", func() (any, error) { return host.InfoWithContext(ctx) })
	report("host.BootTime", func() (any, error) { return host.BootTimeWithContext(ctx) })
	report("host.Uptime", func() (any, error) { return host.UptimeWithContext(ctx) })
	report("host.Users", func() (any, error) { return host.UsersWithContext(ctx) })

	// cpu
	report("cpu.Info", func() (any, error) { return cpu.InfoWithContext(ctx) })
	report("cpu.Counts(logical)", func() (any, error) { return cpu.CountsWithContext(ctx, true) })
	report("cpu.Times(total)", func() (any, error) { return cpu.TimesWithContext(ctx, false) })
	report("cpu.Percent(total)", func() (any, error) { return cpu.PercentWithContext(ctx, 0, false) })

	// mem
	report("mem.VirtualMemory", func() (any, error) { return mem.VirtualMemoryWithContext(ctx) })
	report("mem.SwapMemory", func() (any, error) { return mem.SwapMemoryWithContext(ctx) })
	report("mem.SwapDevices", func() (any, error) { return mem.SwapDevicesWithContext(ctx) })

	// load
	report("load.Avg", func() (any, error) { return load.AvgWithContext(ctx) })
	report("load.Misc", func() (any, error) { return load.MiscWithContext(ctx) })

	// disk
	report("disk.Partitions", func() (any, error) { return disk.PartitionsWithContext(ctx, false) })
	report("disk.Usage(root)", func() (any, error) { return disk.UsageWithContext(ctx, rootPath()) })
	report("disk.IOCounters", func() (any, error) { return disk.IOCountersWithContext(ctx) })

	// net
	report("net.IOCounters(total)", func() (any, error) { return net.IOCountersWithContext(ctx, false) })
	report("net.ProtoCounters", func() (any, error) { return net.ProtoCountersWithContext(ctx, nil) })
	report("net.Connections(all) [count]", func() (any, error) {
		conns, err := net.ConnectionsWithContext(ctx, "all")
		return summary{Count: len(conns)}, err
	})

	// process
	report("process.Pids [count]", func() (any, error) {
		pids, err := process.PidsWithContext(ctx)
		return summary{Count: len(pids)}, err
	})
	reportCurrentProcess(ctx)

	// sensors
	report("sensors.Temperatures", func() (any, error) { return sensors.TemperaturesWithContext(ctx) })

	// nfs (AIX-only; ErrNotImplementedError elsewhere)
	report("nfs.ClientStats", func() (any, error) { return nfs.ClientStatsWithContext(ctx) })
	report("nfs.ServerStats", func() (any, error) { return nfs.ServerStatsWithContext(ctx) })
}

// summary is a compact stand-in for results that would be too large to dump.
type summary struct {
	Count int `json:"count"`
}

// reportCurrentProcess exercises the per-process API against this process,
// including SignalsPending (a fork addition).
func reportCurrentProcess(ctx context.Context) {
	report("process.Current", func() (any, error) {
		p, err := process.NewProcessWithContext(ctx, int32(currentPID()))
		if err != nil {
			return nil, err
		}
		out := map[string]any{}
		out["pid"] = p.Pid
		out["name"], _ = p.NameWithContext(ctx)
		out["exe"], _ = p.ExeWithContext(ctx)
		out["username"], _ = p.UsernameWithContext(ctx)
		if mi, err := p.MemoryInfoWithContext(ctx); err == nil {
			out["memoryInfo"] = mi
		}
		if sig, err := p.SignalsPendingWithContext(ctx); err == nil {
			out["signalsPending"] = sig
		} else {
			out["signalsPending"] = fmt.Sprintf("error: %v", err)
		}
		return out, nil
	})
}

// rootPath returns the platform's root filesystem path for a disk-usage probe.
func rootPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// currentPID returns this process's PID.
func currentPID() int { return os.Getpid() }

// report runs fn and prints its result as JSON, or its error, recovering from
// any panic so a single failing call never aborts the program.
func report(name string, fn func() (any, error)) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("=== %s\n  PANIC: %v\n\n", name, r)
		}
	}()

	v, err := fn()
	if err != nil {
		fmt.Printf("=== %s\n  ERROR: %v\n\n", name, err)
		return
	}
	b, merr := json.MarshalIndent(v, "  ", "  ")
	if merr != nil {
		fmt.Printf("=== %s\n  (unprintable result: %v)\n\n", name, merr)
		return
	}
	fmt.Printf("=== %s\n  %s\n\n", name, string(b))
}
