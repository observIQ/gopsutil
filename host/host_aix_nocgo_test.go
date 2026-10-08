// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package host

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func useFakeLbolt(t *testing.T, lbolt int64) {
	t.Helper()
	orig := perfstatCPUTotalStat
	perfstatCPUTotalStat = func() (*common.PerfstatCPUTotal, error) {
		return &common.PerfstatCPUTotal{Lbolt: lbolt}, nil
	}
	t.Cleanup(func() { perfstatCPUTotalStat = orig })
}

func TestUptimeWithContext_FromLbolt(t *testing.T) {
	// lbolt counts kernel ticks since boot, 100 per second.
	useFakeLbolt(t, 84520439)

	uptime, err := UptimeWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, uint64(845204), uptime)
}

func TestBootTimeWithContext_FromLbolt(t *testing.T) {
	useFakeLbolt(t, 84520439)

	bootTime, err := BootTimeWithContext(context.Background())
	require.NoError(t, err)
	assert.InDelta(t, time.Now().Unix()-845204, int64(bootTime), 2)
}
