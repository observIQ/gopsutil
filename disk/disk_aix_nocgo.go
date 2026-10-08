// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package disk

import (
	"context"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

var startBlank = regexp.MustCompile(`^\s+`)

var ignoreFSType = map[string]bool{"procfs": true}

// perfstatDiskStats reads perfstat_disk, swapped out in tests.
var perfstatDiskStats = common.PerfstatDiskStats

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
	var ret []PartitionStat

	out, err := invoke.CommandWithContext(ctx, "mount")
	if err != nil {
		return nil, err
	}

	// parse head lines for column names
	colidx := make(map[string]int)
	lines := strings.Split(string(out), "\n")
	if len(lines) < 3 {
		return nil, common.ErrNotImplementedError
	}

	idx := 0
	start := 0
	finished := false
	for pos, ch := range lines[1] {
		if ch == ' ' && !finished {
			name := strings.TrimSpace(lines[0][start:pos])
			colidx[name] = idx
			finished = true
		} else if ch == '-' && finished {
			idx++
			start = pos
			finished = false
		}
	}
	name := strings.TrimSpace(lines[0][start:len(lines[1])])
	colidx[name] = idx

	for idx := 2; idx < len(lines); idx++ {
		line := lines[idx]
		if startBlank.MatchString(line) {
			line = "localhost" + line
		}
		p := strings.Fields(line)
		if len(p) < 5 || ignoreFSType[p[colidx["vfs"]]] {
			continue
		}
		d := PartitionStat{
			Device:     p[colidx["mounted"]],
			Mountpoint: p[colidx["mounted over"]],
			Fstype:     p[colidx["vfs"]],
			Opts:       strings.Split(p[colidx["options"]], ","),
		}

		ret = append(ret, d)
	}

	return ret, nil
}

func GetMountFSTypeWithContext(ctx context.Context, mp string) (string, error) {
	out, err := invoke.CommandWithContext(ctx, "mount")
	if err != nil {
		return "", err
	}

	// Kind of inefficient, but it works
	lines := strings.Split(string(out), "\n")
	for line := 1; line < len(lines); line++ {
		fields := strings.Fields(lines[line])
		if strings.TrimSpace(fields[0]) == mp {
			return fields[2], nil
		}
	}

	return "", nil
}
