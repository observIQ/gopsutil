// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package disk

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func useFakeDisks(t *testing.T) {
	t.Helper()
	name := func(s string) (b [64]byte) {
		copy(b[:], s)
		return b
	}
	orig := perfstatDiskStats
	perfstatDiskStats = func() ([]common.PerfstatDisk, error) {
		return []common.PerfstatDisk{
			{
				Name: name("hdisk0"), BSize: 512, XRate: 24141, Xfers: 1243198, RBlks: 976543, WBlks: 26021249,
				QDepth: 2, Time: 174320, RServ: 6213083214, WServ: 262578175892, WQTime: 5000000,
			},
			{Name: name("cd0"), BSize: 2048, XRate: 18, Xfers: 18, RBlks: 26},
		}, nil
	}
	t.Cleanup(func() { perfstatDiskStats = orig })
}

func TestIOCountersWithContext_FromPerfstat(t *testing.T) {
	useFakeDisks(t)

	stats, err := IOCountersWithContext(context.Background())
	require.NoError(t, err)
	assert.Equal(t, IOCountersStat{
		Name:           "hdisk0",
		ReadCount:      24141,
		WriteCount:     1243198 - 24141,
		ReadBytes:      976543 * 512,
		WriteBytes:     26021249 * 512,
		ReadTime:       6213,        // ns to ms
		WriteTime:      262578,      // ns to ms
		IoTime:         174320 * 10, // 100 ticks per second, to ms
		WeightedIO:     5,           // ns to ms
		IopsInProgress: 2,
	}, stats["hdisk0"])
	assert.Equal(t, uint64(26*2048), stats["cd0"].ReadBytes)
}

func TestIOCountersWithContext_FiltersNames(t *testing.T) {
	useFakeDisks(t)

	stats, err := IOCountersWithContext(context.Background(), "cd0")
	require.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Contains(t, stats, "cd0")
}

func useFakeMounts(t *testing.T) {
	t.Helper()
	t.Setenv("HOST_ETC", "testdata/aix") // vfs copied from AIX 7.3
	orig := aixMounts
	aixMounts = func() ([]common.AIXMount, error) {
		return []common.AIXMount{
			{Object: "/dev/hd4", Stub: "/", Host: "-", Args: "rw,log=/dev/hd8", GfsType: 0},
			{Object: "/proc", Stub: "/proc", Host: "-", Args: "rw", GfsType: 6},
			{Object: "/dev/hd1", Stub: "/home", Host: "-", Args: "rw,noatime,log=/dev/hd8", GfsType: 0},
			{Object: "/export/data", Stub: "/mnt/data", Host: "filer", Args: "ro,bg,hard", GfsType: 18},
		}, nil
	}
	t.Cleanup(func() { aixMounts = orig })
}

func TestPartitionsWithContext_FromMntctl(t *testing.T) {
	useFakeMounts(t)

	parts, err := PartitionsWithContext(context.Background(), true)
	require.NoError(t, err)
	// procfs is left out, as before.
	assert.Equal(t, []PartitionStat{
		{Device: "/dev/hd4", Mountpoint: "/", Fstype: "jfs2", Opts: []string{"rw", "log=/dev/hd8"}},
		{Device: "/dev/hd1", Mountpoint: "/home", Fstype: "jfs2", Opts: []string{"rw", "noatime", "log=/dev/hd8"}},
		{Device: "/export/data", Mountpoint: "/mnt/data", Fstype: "nfs3", Opts: []string{"ro", "bg", "hard"}},
	}, parts)
}

func TestGetMountFSTypeWithContext_FromMntctl(t *testing.T) {
	useFakeMounts(t)

	fstype, err := GetMountFSTypeWithContext(context.Background(), "/mnt/data")
	require.NoError(t, err)
	assert.Equal(t, "nfs3", fstype)
}
