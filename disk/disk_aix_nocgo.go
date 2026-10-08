// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package disk

import (
	"context"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

var ignoreFSType = map[string]bool{"procfs": true}

// perfstat and libc reads, swapped out in tests.
var (
	perfstatDiskStats = common.PerfstatDiskStats
	aixMounts         = common.AIXMounts
)

func IOCountersWithContext(_ context.Context, names ...string) (map[string]IOCountersStat, error) {
	disks, err := perfstatDiskStats()
	if err != nil {
		return nil, err
	}
	ret := make(map[string]IOCountersStat, len(disks))
	for i := range disks {
		d := &disks[i]
		name := common.CString(d.Name[:])
		if len(names) > 0 && !common.StringsHas(names, name) {
			continue
		}
		// The same conversions as the cgo build: service and queue times are in
		// nanoseconds, busy time in kernel ticks.
		ret[name] = IOCountersStat{
			Name:           name,
			ReadCount:      d.XRate,
			WriteCount:     d.Xfers - d.XRate,
			ReadBytes:      d.RBlks * d.BSize,
			WriteBytes:     d.WBlks * d.BSize,
			ReadTime:       d.RServ / 1_000_000,
			WriteTime:      d.WServ / 1_000_000,
			IoTime:         d.Time * 1000 / common.AIXClockTicks,
			WeightedIO:     d.WQTime / 1_000_000,
			IopsInProgress: d.QDepth,
		}
	}
	return ret, nil
}

func PartitionsWithContext(ctx context.Context, _ bool) ([]PartitionStat, error) {
	mounts, err := aixMounts()
	if err != nil {
		return nil, err
	}
	names := vfsNames(ctx)
	var ret []PartitionStat
	for _, m := range mounts {
		fstype := names[m.GfsType]
		if ignoreFSType[fstype] {
			continue
		}
		ret = append(ret, PartitionStat{
			Device:     m.Object,
			Mountpoint: m.Stub,
			Fstype:     fstype,
			Opts:       strings.Split(m.Args, ","),
		})
	}
	return ret, nil
}

func GetMountFSTypeWithContext(ctx context.Context, mp string) (string, error) {
	mounts, err := aixMounts()
	if err != nil {
		return "", err
	}
	for _, m := range mounts {
		if m.Stub == mp {
			return vfsNames(ctx)[m.GfsType], nil
		}
	}
	return "", nil
}

// vfsNames maps filesystem type numbers to names from /etc/vfs, as mount does.
// Each entry line reads "name number mount_helper filsys_helper [remote]".
func vfsNames(ctx context.Context) map[int32]string {
	names := map[int32]string{}
	lines, _ := common.ReadLines(common.HostEtcWithContext(ctx, "vfs"))
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) < 2 || strings.HasPrefix(f[0], "#") || strings.HasPrefix(f[0], "%") {
			continue
		}
		if n, err := strconv.ParseInt(f[1], 10, 32); err == nil {
			names[int32(n)] = f[0]
		}
	}
	return names
}
