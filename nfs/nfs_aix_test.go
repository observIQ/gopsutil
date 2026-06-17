// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package nfs

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseNfsstat(t *testing.T) {
	output, err := os.ReadFile("testdata/nfsstat_aix.txt")
	require.NoError(t, err)

	stats := &StatsStat{
		NFSClientStats: NFSClientStat{Operations: make(map[string]uint64)},
		NFSServerStats: NFSServerStat{Operations: make(map[string]uint64)},
	}
	parseNfsstat(string(output), stats)

	// Client RPC calls are summed across transports; the fixture has 12 on the
	// connectionless (UDP) transport and 0 on connection-oriented (TCP).
	assert.Equal(t, uint64(12), stats.RPCClientStats.Calls)
	assert.Equal(t, uint64(0), stats.RPCServerStats.Calls)

	// Operation keys must be the real operation names with a version prefix,
	// not the collapsed "v2_0" garbage the previous one-line parser produced.
	assert.Equal(t, uint64(0), stats.NFSServerStats.Operations["v2_null"])
	for _, key := range []string{"v2_null", "v2_getattr", "v2_statfs", "v3_access", "v3_commit", "v3_readdir+"} {
		_, ok := stats.NFSServerStats.Operations[key]
		assert.Truef(t, ok, "expected server operation key %q", key)
	}
	for _, key := range []string{"v2_null", "v3_getattr", "v3_pathconf"} {
		_, ok := stats.NFSClientStats.Operations[key]
		assert.Truef(t, ok, "expected client operation key %q", key)
	}

	// v2 has 18 operations and v3 has 22, on both server and client.
	assert.Len(t, stats.NFSServerStats.Operations, 40)
	assert.Len(t, stats.NFSClientStats.Operations, 40)

	// No collapsed numeric-only key should exist.
	_, bad := stats.NFSServerStats.Operations["v2_0"]
	assert.False(t, bad, "parser produced a collapsed numeric operation key")
}

func TestClientStatsWithContext(t *testing.T) {
	ctx := context.Background()
	stats, err := ClientStatsWithContext(ctx)
	if err != nil {
		// NFS may not be configured on the host; the parse path is covered by
		// TestParseNfsstat, so a live failure here is acceptable.
		t.Logf("NFS client stats not available: %v", err)
		return
	}
	assert.NotNil(t, stats)
}

func TestServerStatsWithContext(t *testing.T) {
	ctx := context.Background()
	stats, err := ServerStatsWithContext(ctx)
	if err != nil {
		t.Logf("NFS server stats not available: %v", err)
		return
	}
	assert.NotNil(t, stats)
}
