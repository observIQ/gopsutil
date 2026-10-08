// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package common

import (
	"os"
	"slices"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The expected sizes and offsets come from gcc's sizeof/offsetof against libperfstat.h,
// identical on AIX 7.2 TL5 and 7.3 TL4.
func TestPerfstatLayouts(t *testing.T) {
	assert.Equal(t, uintptr(640), unsafe.Sizeof(PerfstatCPU{}))
	assert.Equal(t, uintptr(64), unsafe.Offsetof(PerfstatCPU{}.User))
	assert.Equal(t, uintptr(80), unsafe.Offsetof(PerfstatCPU{}.Idle))
	assert.Equal(t, uintptr(88), unsafe.Offsetof(PerfstatCPU{}.Wait))

	assert.Equal(t, uintptr(696), unsafe.Sizeof(PerfstatCPUTotal{}))
	assert.Equal(t, uintptr(8), unsafe.Offsetof(PerfstatCPUTotal{}.Description))
	assert.Equal(t, uintptr(80), unsafe.Offsetof(PerfstatCPUTotal{}.User))
	assert.Equal(t, uintptr(96), unsafe.Offsetof(PerfstatCPUTotal{}.Idle))
	assert.Equal(t, uintptr(104), unsafe.Offsetof(PerfstatCPUTotal{}.Wait))
	assert.Equal(t, uintptr(112), unsafe.Offsetof(PerfstatCPUTotal{}.PSwitch))
	assert.Equal(t, uintptr(120), unsafe.Offsetof(PerfstatCPUTotal{}.Syscall))
	assert.Equal(t, uintptr(176), unsafe.Offsetof(PerfstatCPUTotal{}.DevIntrs))
	assert.Equal(t, uintptr(192), unsafe.Offsetof(PerfstatCPUTotal{}.Lbolt))
	assert.Equal(t, uintptr(200), unsafe.Offsetof(PerfstatCPUTotal{}.LoadAvg))

	assert.Equal(t, uintptr(496), unsafe.Sizeof(PerfstatDisk{}))
	assert.Equal(t, uintptr(208), unsafe.Offsetof(PerfstatDisk{}.BSize))
	assert.Equal(t, uintptr(216), unsafe.Offsetof(PerfstatDisk{}.XRate))
	assert.Equal(t, uintptr(224), unsafe.Offsetof(PerfstatDisk{}.Xfers))
	assert.Equal(t, uintptr(232), unsafe.Offsetof(PerfstatDisk{}.WBlks))
	assert.Equal(t, uintptr(240), unsafe.Offsetof(PerfstatDisk{}.RBlks))
	assert.Equal(t, uintptr(248), unsafe.Offsetof(PerfstatDisk{}.QDepth))
	assert.Equal(t, uintptr(256), unsafe.Offsetof(PerfstatDisk{}.Time))
	assert.Equal(t, uintptr(344), unsafe.Offsetof(PerfstatDisk{}.RServ))
	assert.Equal(t, uintptr(384), unsafe.Offsetof(PerfstatDisk{}.WServ))
	assert.Equal(t, uintptr(440), unsafe.Offsetof(PerfstatDisk{}.WQTime))

	assert.Equal(t, uintptr(240), unsafe.Sizeof(PerfstatNetInterface{}))
	assert.Equal(t, uintptr(144), unsafe.Offsetof(PerfstatNetInterface{}.IPackets))
	assert.Equal(t, uintptr(152), unsafe.Offsetof(PerfstatNetInterface{}.IBytes))
	assert.Equal(t, uintptr(160), unsafe.Offsetof(PerfstatNetInterface{}.IErrors))
	assert.Equal(t, uintptr(168), unsafe.Offsetof(PerfstatNetInterface{}.OPackets))
	assert.Equal(t, uintptr(176), unsafe.Offsetof(PerfstatNetInterface{}.OBytes))
	assert.Equal(t, uintptr(184), unsafe.Offsetof(PerfstatNetInterface{}.OErrors))
	assert.Equal(t, uintptr(208), unsafe.Offsetof(PerfstatNetInterface{}.XmitDrops))
	assert.Equal(t, uintptr(224), unsafe.Offsetof(PerfstatNetInterface{}.IfIqDrops))

	assert.Equal(t, uintptr(352), unsafe.Sizeof(PerfstatMemoryTotal{}))
	assert.Equal(t, uintptr(8), unsafe.Offsetof(PerfstatMemoryTotal{}.RealTotal))
	assert.Equal(t, uintptr(16), unsafe.Offsetof(PerfstatMemoryTotal{}.RealFree))
	assert.Equal(t, uintptr(32), unsafe.Offsetof(PerfstatMemoryTotal{}.RealInUse))
	assert.Equal(t, uintptr(56), unsafe.Offsetof(PerfstatMemoryTotal{}.PgIns))
	assert.Equal(t, uintptr(64), unsafe.Offsetof(PerfstatMemoryTotal{}.PgOuts))
	assert.Equal(t, uintptr(72), unsafe.Offsetof(PerfstatMemoryTotal{}.PgSpIns))
	assert.Equal(t, uintptr(80), unsafe.Offsetof(PerfstatMemoryTotal{}.PgSpOuts))
	assert.Equal(t, uintptr(120), unsafe.Offsetof(PerfstatMemoryTotal{}.PgSpTotal))
	assert.Equal(t, uintptr(128), unsafe.Offsetof(PerfstatMemoryTotal{}.PgSpFree))
	assert.Equal(t, uintptr(168), unsafe.Offsetof(PerfstatMemoryTotal{}.VirtActive))
	assert.Equal(t, uintptr(328), unsafe.Offsetof(PerfstatMemoryTotal{}.RealAvail))

	assert.Equal(t, uintptr(248), unsafe.Sizeof(PerfstatPagingSpace{}))
	assert.Equal(t, uintptr(208), unsafe.Offsetof(PerfstatPagingSpace{}.MBSize))
	assert.Equal(t, uintptr(216), unsafe.Offsetof(PerfstatPagingSpace{}.MBUsed))

	assert.Equal(t, uintptr(5024), unsafe.Sizeof(AIXProcess{}))
	assert.Equal(t, uintptr(52), unsafe.Offsetof(AIXProcess{}.State))
	assert.Equal(t, uintptr(56), unsafe.Offsetof(AIXProcess{}.Flags))
	assert.Equal(t, uintptr(144), unsafe.Offsetof(AIXProcess{}.MajFlt))
	assert.Equal(t, uintptr(1504), unsafe.Offsetof(AIXProcess{}.IOChars))

	assert.Equal(t, uintptr(424), unsafe.Sizeof(AIXThread{}))
	assert.Equal(t, uintptr(8), unsafe.Offsetof(AIXThread{}.Pid))
	assert.Equal(t, uintptr(20), unsafe.Offsetof(AIXThread{}.State))
	assert.Equal(t, uintptr(24), unsafe.Offsetof(AIXThread{}.Flags))
	assert.Equal(t, uintptr(32), unsafe.Offsetof(AIXThread{}.WaitType))
}

func TestPerfstatCPUStats_Live(t *testing.T) {
	total, err := PerfstatCPUTotalStat()
	require.NoError(t, err)
	assert.Positive(t, total.NCPUs)
	assert.Positive(t, total.Lbolt)
	assert.NotEmpty(t, CString(total.Description[:]))

	cpus, err := PerfstatCPUStats()
	require.NoError(t, err)
	require.Len(t, cpus, int(total.NCPUs))
	assert.Equal(t, "cpu0", CString(cpus[0].Name[:]))

	var idle uint64
	for i := range cpus {
		idle += cpus[i].Idle
	}
	// Both reads are cumulative, so the per-CPU sum taken second can only be at or above the total.
	assert.GreaterOrEqual(t, idle, total.Idle)
}

func TestAIXProcessesAndThreads_Live(t *testing.T) {
	self := int32(os.Getpid())

	procs, err := AIXProcesses()
	require.NoError(t, err)
	assert.True(t, slices.ContainsFunc(procs, func(p AIXProcess) bool { return p.Pid == self }), "own process listed")
	assert.True(t, slices.ContainsFunc(procs, func(p AIXProcess) bool { return p.Flags&AIXProcKernel != 0 }), "kernel processes listed")

	threads, err := AIXThreads()
	require.NoError(t, err)
	assert.True(t, slices.ContainsFunc(threads, func(th AIXThread) bool { return th.Pid == self && th.State == AIXThreadRunnable }),
		"own thread listed as runnable")
}

func TestPerfstatDiskStats_Live(t *testing.T) {
	disks, err := PerfstatDiskStats()
	require.NoError(t, err)
	i := slices.IndexFunc(disks, func(d PerfstatDisk) bool { return CString(d.Name[:]) == "hdisk0" })
	require.GreaterOrEqual(t, i, 0, "hdisk0 listed")
	assert.Equal(t, uint64(512), disks[i].BSize)
	assert.Positive(t, disks[i].Xfers)
}

func TestPerfstatNetInterfaceStats_Live(t *testing.T) {
	ifs, err := PerfstatNetInterfaceStats()
	require.NoError(t, err)
	i := slices.IndexFunc(ifs, func(n PerfstatNetInterface) bool { return CString(n.Name[:]) == "lo0" })
	require.GreaterOrEqual(t, i, 0, "lo0 listed")
	assert.Positive(t, ifs[i].IBytes)
}

func TestPerfstatMemory_Live(t *testing.T) {
	m, err := PerfstatMemoryTotalStat()
	require.NoError(t, err)
	assert.Positive(t, m.RealTotal)
	assert.LessOrEqual(t, m.RealFree, m.RealTotal)

	spaces, err := PerfstatPagingSpaceStats()
	require.NoError(t, err)
	require.NotEmpty(t, spaces)
	assert.Positive(t, spaces[0].MBSize)
}

func TestAIXMounts_Live(t *testing.T) {
	mounts, err := AIXMounts()
	require.NoError(t, err)
	i := slices.IndexFunc(mounts, func(m AIXMount) bool { return m.Stub == "/" })
	require.GreaterOrEqual(t, i, 0, "root filesystem listed")
	assert.Equal(t, "/dev/hd4", mounts[i].Object)
	assert.Equal(t, int32(0), mounts[i].GfsType, "jfs2")
	assert.Contains(t, mounts[i].Args, "rw")
}

func TestAIXProcessEnv_Live(t *testing.T) {
	// getevars returns the environment the process started with, like ps eww.
	env, err := AIXProcessEnv(int32(os.Getpid()))
	require.NoError(t, err)
	assert.Contains(t, env, "HOME="+os.Getenv("HOME"))
}
