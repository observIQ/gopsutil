// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package common

// gopsutil calls libperfstat and libc directly, without cgo, the way golang.org/x/sys/unix
// calls libc on AIX: cgo_import_dynamic binds the symbol, linkname exposes it, and the
// runtime's syscall6 (reached through asm_aix_ppc64.s) makes the call.

import (
	"encoding/binary"
	"errors"
	"syscall"
	"unsafe"
)

//go:cgo_import_dynamic libperfstat_cpu perfstat_cpu "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libperfstat_cpu_total perfstat_cpu_total "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libperfstat_disk perfstat_disk "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libperfstat_netinterface perfstat_netinterface "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libperfstat_memory_total perfstat_memory_total "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libperfstat_pagingspace perfstat_pagingspace "libperfstat.a/shr_64.o"
//go:cgo_import_dynamic libc_mntctl mntctl "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_getprocs64 getprocs64 "libc.a/shr_64.o"
//go:cgo_import_dynamic libc_getthrds64 getthrds64 "libc.a/shr_64.o"

//go:linkname libperfstat_cpu libperfstat_cpu
//go:linkname libperfstat_cpu_total libperfstat_cpu_total
//go:linkname libperfstat_disk libperfstat_disk
//go:linkname libperfstat_netinterface libperfstat_netinterface
//go:linkname libperfstat_memory_total libperfstat_memory_total
//go:linkname libperfstat_pagingspace libperfstat_pagingspace
//go:linkname libc_mntctl libc_mntctl
//go:linkname libc_getprocs64 libc_getprocs64
//go:linkname libc_getthrds64 libc_getthrds64

type libFunc uintptr

//nolint:revive // names must match the linkname symbols
var libperfstat_cpu, libperfstat_cpu_total, libperfstat_disk, libperfstat_netinterface,
	libperfstat_memory_total, libperfstat_pagingspace, libc_mntctl, libc_getprocs64, libc_getthrds64 libFunc

// Implemented in the runtime (syscall/syscall_aix.go), reached through asm_aix_ppc64.s.
func syscall6(trap, nargs, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)

// callLib calls an imported C function and returns its int result, or the errno when it
// returns a negative value. uintptrescapes keeps pointer arguments on the heap and alive
// for the duration of the call.
//
//go:uintptrescapes
func callLib(fn *libFunc, nargs int, a1, a2, a3, a4, a5, a6 uintptr) (int, error) {
	r, _, e := syscall6(uintptr(unsafe.Pointer(fn)), uintptr(nargs), a1, a2, a3, a4, a5, a6)
	if n := int(int32(r)); n >= 0 {
		return n, nil
	}
	if e != 0 {
		return 0, e
	}
	return 0, errors.New("libperfstat call failed")
}

// AIXClockTicks is the AIX kernel tick rate (HZ), which perfstat tick counters use.
const AIXClockTicks = 100

// AIXLoadAvgScale is 1<<SBITS from sys/proc.h, the fixed-point scale of perfstat load averages.
const AIXLoadAvgScale = 1 << 16

// PerfstatCPU is the leading part of perfstat_cpu_t from libperfstat.h. Offsets and the
// 640-byte size were checked with gcc on AIX 7.2 and 7.3.
type PerfstatCPU struct {
	Name [64]byte
	User uint64
	Sys  uint64
	Idle uint64
	Wait uint64
	_    [640 - 96]byte
}

// PerfstatCPUTotal is the part of perfstat_cpu_total_t from libperfstat.h that gopsutil
// reads. Offsets and the 696-byte size were checked with gcc on AIX 7.2 and 7.3.
type PerfstatCPUTotal struct {
	NCPUs       int32
	NCPUsCfg    int32
	Description [64]byte
	ProcessorHz uint64
	User        uint64
	Sys         uint64
	Idle        uint64
	Wait        uint64
	PSwitch     uint64 // context switches
	Syscall     uint64
	_           [176 - 128]byte
	DevIntrs    uint64 // device interrupts
	_           [192 - 184]byte
	Lbolt       int64
	LoadAvg     [3]uint64
	_           [696 - 224]byte
}

// PerfstatCPUTotalStat reads perfstat_cpu_total.
func PerfstatCPUTotalStat() (*PerfstatCPUTotal, error) {
	t := new(PerfstatCPUTotal)
	if _, err := callLib(&libperfstat_cpu_total, 4, 0, uintptr(unsafe.Pointer(t)), unsafe.Sizeof(*t), 1, 0, 0); err != nil {
		return nil, err
	}
	return t, nil
}

// PerfstatCPUStats reads perfstat_cpu for every logical CPU.
func PerfstatCPUStats() ([]PerfstatCPU, error) {
	return perfstatEnumerate[PerfstatCPU](&libperfstat_cpu)
}

// perfstatEnumerate reads every entry of a perfstat function that lists items (CPUs,
// disks, interfaces): a first call counts them, a second fills a buffer from the first.
func perfstatEnumerate[T any](fn *libFunc) ([]T, error) {
	var zero T
	size := unsafe.Sizeof(zero)
	n, err := callLib(fn, 4, 0, 0, size, 0, 0, 0)
	if err != nil || n == 0 {
		return nil, err
	}
	buf := make([]T, n)
	first := new([64]byte) // an empty name starts at the first entry
	n, err = callLib(fn, 4, uintptr(unsafe.Pointer(first)), uintptr(unsafe.Pointer(&buf[0])), size, uintptr(n), 0, 0)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// PerfstatDisk is the part of perfstat_disk_t from libperfstat.h that gopsutil reads.
// Offsets and the 496-byte size were checked with gcc on AIX 7.2 and 7.3.
type PerfstatDisk struct {
	Name   [64]byte
	_      [208 - 64]byte
	BSize  uint64
	XRate  uint64 // reads
	Xfers  uint64 // reads plus writes
	WBlks  uint64
	RBlks  uint64
	QDepth uint64
	Time   uint64 // busy ticks
	_      [344 - 264]byte
	RServ  uint64 // read service time, ns
	_      [384 - 352]byte
	WServ  uint64 // write service time, ns
	_      [440 - 392]byte
	WQTime uint64 // wait queue time, ns
	_      [496 - 448]byte
}

// PerfstatDiskStats reads perfstat_disk for every disk.
func PerfstatDiskStats() ([]PerfstatDisk, error) {
	return perfstatEnumerate[PerfstatDisk](&libperfstat_disk)
}

// PerfstatNetInterface is the part of perfstat_netinterface_t from libperfstat.h that
// gopsutil reads. Offsets and the 240-byte size were checked with gcc on AIX 7.2 and 7.3.
type PerfstatNetInterface struct {
	Name      [64]byte
	_         [144 - 64]byte
	IPackets  uint64
	IBytes    uint64
	IErrors   uint64
	OPackets  uint64
	OBytes    uint64
	OErrors   uint64
	_         [208 - 192]byte
	XmitDrops uint64
	_         [224 - 216]byte
	IfIqDrops uint64
	_         [240 - 232]byte
}

// PerfstatNetInterfaceStats reads perfstat_netinterface for every interface.
func PerfstatNetInterfaceStats() ([]PerfstatNetInterface, error) {
	return perfstatEnumerate[PerfstatNetInterface](&libperfstat_netinterface)
}

// PerfstatMemoryTotal is the part of perfstat_memory_total_t from libperfstat.h that
// gopsutil reads. Counts are 4 KB pages. Offsets and the 352-byte size were checked with
// gcc on AIX 7.2 and 7.3.
type PerfstatMemoryTotal struct {
	_          [8]byte
	RealTotal  uint64
	RealFree   uint64
	_          [32 - 24]byte
	RealInUse  uint64
	_          [56 - 40]byte
	PgIns      uint64
	PgOuts     uint64
	PgSpIns    uint64
	PgSpOuts   uint64
	_          [120 - 88]byte
	PgSpTotal  uint64
	PgSpFree   uint64
	_          [168 - 136]byte
	VirtActive uint64
	_          [328 - 176]byte
	RealAvail  uint64
	_          [352 - 336]byte
}

// PerfstatPagingSpace is the part of perfstat_pagingspace_t from libperfstat.h that
// gopsutil reads. Offsets and the 248-byte size were checked with gcc on AIX 7.2 and 7.3.
type PerfstatPagingSpace struct {
	Name   [64]byte
	_      [208 - 64]byte
	MBSize int64
	MBUsed int64
	_      [248 - 224]byte
}

// PerfstatMemoryTotalStat reads perfstat_memory_total.
func PerfstatMemoryTotalStat() (*PerfstatMemoryTotal, error) {
	m := new(PerfstatMemoryTotal)
	if _, err := callLib(&libperfstat_memory_total, 4, 0, uintptr(unsafe.Pointer(m)), unsafe.Sizeof(*m), 1, 0, 0); err != nil {
		return nil, err
	}
	return m, nil
}

// PerfstatPagingSpaceStats reads perfstat_pagingspace for every paging space.
func PerfstatPagingSpaceStats() ([]PerfstatPagingSpace, error) {
	return perfstatEnumerate[PerfstatPagingSpace](&libperfstat_pagingspace)
}

// AIXMount is one entry of the mount table from mntctl(MCTL_QUERY).
type AIXMount struct {
	Object  string // the mounted device or remote path
	Stub    string // the mount point
	Host    string // "-" for a local mount
	Args    string // mount options, as mount prints them
	GfsType int32  // filesystem type number, named in /etc/vfs
}

// AIXMounts reads the mount table through mntctl.
func AIXMounts() ([]AIXMount, error) {
	const mctlQuery = 2 // MCTL_QUERY in sys/vmount.h
	buf := make([]byte, 8192)
	n, err := callLib(&libc_mntctl, 3, mctlQuery, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), 0, 0, 0)
	if err == nil && n == 0 {
		// The buffer was too small; mntctl put the size it needs in the first word.
		buf = make([]byte, binary.BigEndian.Uint32(buf))
		n, err = callLib(&libc_mntctl, 3, mctlQuery, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), 0, 0, 0)
	}
	if err != nil {
		return nil, err
	}
	return parseVmounts(buf, n), nil
}

// parseVmounts reads n struct vmount records from sys/vmount.h. Each record holds
// vmt_length at offset 4, vmt_gfstype at 40, and from 44 an array of {offset, size}
// int16 pairs locating its strings, all checked with gcc on AIX 7.2 and 7.3.
func parseVmounts(buf []byte, n int) []AIXMount {
	const (
		vmtObject, vmtStub, vmtHost, vmtArgs = 0, 1, 2, 5
		vmtData                              = 44
	)
	ret := make([]AIXMount, 0, n)
	for off := 0; len(ret) < n && off+vmtData+6*4 <= len(buf); {
		rec := buf[off:]
		length := int(binary.BigEndian.Uint32(rec[4:]))
		if length <= 0 || off+length > len(buf) {
			break
		}
		rec = rec[:length]
		str := func(i int) string {
			o := int(binary.BigEndian.Uint16(rec[vmtData+4*i:]))
			size := int(binary.BigEndian.Uint16(rec[vmtData+4*i+2:]))
			if o+size > len(rec) {
				return ""
			}
			return CString(rec[o : o+size])
		}
		ret = append(ret, AIXMount{
			Object:  str(vmtObject),
			Stub:    str(vmtStub),
			Host:    str(vmtHost),
			Args:    str(vmtArgs),
			GfsType: int32(binary.BigEndian.Uint32(rec[40:])),
		})
		off += length
	}
	return ret
}

// AIXProcess is the part of procentry64 from procinfo.h that gopsutil reads through
// getprocs64. Offsets and the 5024-byte size were checked with gcc on AIX 7.2 and 7.3.
type AIXProcess struct {
	Pid     int32
	_       [52 - 4]byte
	State   uint32
	Flags   uint32
	_       [144 - 60]byte
	MajFlt  int64
	_       [1504 - 152]byte
	IOChars int64
	_       [5024 - 1512]byte
}

// AIXThread is the part of thrdentry64 from procinfo.h that gopsutil reads through
// getthrds64. Offsets and the 424-byte size were checked with gcc on AIX 7.2 and 7.3.
type AIXThread struct {
	Tid      uint64
	Pid      int32
	_        [20 - 12]byte
	State    uint32
	Flags    uint32
	_        [32 - 28]byte
	WaitType uint32
	_        [424 - 36]byte
}

// Process and thread flags and states from sys/proc.h and sys/thread.h.
const (
	AIXProcKernel     = 0x200  // SKPROC
	AIXThreadKernel   = 0x1000 // TKTHREAD
	AIXThreadRunnable = 2      // TSRUN
	AIXThreadSleeping = 3      // TSSLEEP
)

// AIXVMMWait reports whether a sleeping thread waits in the VMM wait queue (TWPGIN,
// TWPGOUT, TWFREEF or TWMEM), which is what vmstat reports as blocked.
func AIXVMMWait(waitType uint32) bool {
	switch waitType {
	case 5, 6, 8, 9:
		return true
	}
	return false
}

// AIXProcesses reads every process through getprocs64.
func AIXProcesses() ([]AIXProcess, error) {
	const batch = 256
	var ret []AIXProcess
	index := new(int32) // getprocs64 resumes after this pid
	for {
		buf := make([]AIXProcess, batch)
		n, err := callLib(&libc_getprocs64, 6, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]), 0, 0, uintptr(unsafe.Pointer(index)), batch)
		if err != nil {
			return nil, err
		}
		ret = append(ret, buf[:n]...)
		if n < batch {
			return ret, nil
		}
	}
}

// AIXThreads reads every thread on the system through getthrds64.
func AIXThreads() ([]AIXThread, error) {
	const batch = 256
	allProcesses := uintptr(^uint32(0)) // pid -1
	var ret []AIXThread
	index := new(int64) // getthrds64 resumes after this thread id
	for {
		buf := make([]AIXThread, batch)
		n, err := callLib(&libc_getthrds64, 5, allProcesses, uintptr(unsafe.Pointer(&buf[0])), unsafe.Sizeof(buf[0]), uintptr(unsafe.Pointer(index)), batch, 0)
		if err != nil {
			return nil, err
		}
		ret = append(ret, buf[:n]...)
		if n < batch {
			return ret, nil
		}
	}
}

// CString returns the NUL-terminated string at the start of b.
func CString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
