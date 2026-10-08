// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package net

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func useFakeInterfaces(t *testing.T) {
	t.Helper()
	name := func(s string) (b [64]byte) {
		copy(b[:], s)
		return b
	}
	orig := perfstatNetInterfaceStats
	perfstatNetInterfaceStats = func() ([]common.PerfstatNetInterface, error) {
		return []common.PerfstatNetInterface{
			{
				Name: name("en0"), IPackets: 3860778, IBytes: 3519464071, IErrors: 1, OPackets: 1184991,
				OBytes: 70638108, OErrors: 2, XmitDrops: 3, IfIqDrops: 4,
			},
			{Name: name("lo0"), IPackets: 171260, IBytes: 12189943, OPackets: 171260, OBytes: 12189943},
		}, nil
	}
	t.Cleanup(func() { perfstatNetInterfaceStats = orig })
}

func TestIOCountersWithContext_PerNICFromPerfstat(t *testing.T) {
	useFakeInterfaces(t)

	stats, err := IOCountersWithContext(context.Background(), true)
	require.NoError(t, err)
	assert.Equal(t, []IOCountersStat{
		{
			Name: "en0", BytesSent: 70638108, BytesRecv: 3519464071, PacketsSent: 1184991, PacketsRecv: 3860778,
			Errin: 1, Errout: 2, Dropin: 4, Dropout: 3,
		},
		{Name: "lo0", BytesSent: 12189943, BytesRecv: 12189943, PacketsSent: 171260, PacketsRecv: 171260},
	}, stats)
}

func TestIOCountersWithContext_AllFromPerfstat(t *testing.T) {
	useFakeInterfaces(t)

	stats, err := IOCountersWithContext(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	assert.Equal(t, uint64(3519464071+12189943), stats[0].BytesRecv)
}
