// SPDX-License-Identifier: BSD-3-Clause
//go:build aix && !cgo

package common

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
	_           [192 - 112]byte
	Lbolt       int64
	LoadAvg     [3]uint64
	_           [696 - 224]byte
}

// PerfstatCPUTotalStat reads perfstat_cpu_total.
func PerfstatCPUTotalStat() (*PerfstatCPUTotal, error) {
	return nil, ErrNotImplementedError
}

// PerfstatCPUStats reads perfstat_cpu for every logical CPU.
func PerfstatCPUStats() ([]PerfstatCPU, error) {
	return nil, ErrNotImplementedError
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
