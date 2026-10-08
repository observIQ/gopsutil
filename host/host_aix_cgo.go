// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && cgo

package host

import (
	"context"

	"github.com/power-devops/perfstat"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func BootTimeWithContext(ctx context.Context) (btime uint64, err error) {
	return common.BootTimeWithContext(ctx, getInvoker())
}

// Uses ps to get the elapsed time for PID 1 in DAYS-HOURS:MINUTES:SECONDS format.
func UptimeWithContext(ctx context.Context) (uint64, error) {
	return common.UptimeWithContext(ctx, getInvoker())
}

func numProcs(_ context.Context) (uint64, error) {
	procs, err := perfstat.ProcessStat()
	if err != nil {
		return 0, err
	}
	return uint64(len(procs)), nil
}
