// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package host

import (
	"context"

	"github.com/shirou/gopsutil/v4/internal/common"
	"github.com/shirou/gopsutil/v4/process"
)

// perfstatCPUTotalStat reads perfstat_cpu_total, swapped out in tests.
var perfstatCPUTotalStat = common.PerfstatCPUTotalStat

func numProcs(ctx context.Context) (uint64, error) {
	procs, err := process.PidsWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(len(procs)), nil
}
