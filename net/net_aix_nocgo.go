// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package net

import (
	"context"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// perfstatNetInterfaceStats reads perfstat_netinterface, swapped out in tests.
var perfstatNetInterfaceStats = common.PerfstatNetInterfaceStats

func IOCountersWithContext(_ context.Context, pernic bool) ([]IOCountersStat, error) {
	ifs, err := perfstatNetInterfaceStats()
	if err != nil {
		return nil, err
	}
	iocounters := make([]IOCountersStat, 0, len(ifs))
	for i := range ifs {
		n := &ifs[i]
		iocounters = append(iocounters, IOCountersStat{
			Name:        common.CString(n.Name[:]),
			BytesSent:   n.OBytes,
			BytesRecv:   n.IBytes,
			PacketsSent: n.OPackets,
			PacketsRecv: n.IPackets,
			Errin:       n.IErrors,
			Errout:      n.OErrors,
			Dropin:      n.IfIqDrops,
			Dropout:     n.XmitDrops,
		})
	}
	if !pernic {
		return getIOCountersAll(iocounters), nil
	}
	return iocounters, nil
}
