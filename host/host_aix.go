// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package host

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v4/internal/common"
)

// from https://www.ibm.com/docs/en/aix/7.2?topic=files-utmph-file
const (
	user_PROCESS = 7 //nolint:revive //FIXME
)

// utmpPath is the active-sessions database read by UsersWithContext.
// It is a variable so tests can point it at a fixture file.
var utmpPath = "/etc/utmp"

// testInvoker is used for dependency injection in tests
var testInvoker common.Invoker

// getInvoker returns the test invoker if set, otherwise returns the default
func getInvoker() common.Invoker {
	if testInvoker != nil {
		return testInvoker
	}
	return invoke
}

// Static host information is cached because these values (hardware ID,
// platform name, OS version, kernel version, architecture) do not change
// at runtime. Caching avoids spawning subprocesses on repeated queries.
// Only successful lookups are cached, so a transient failure (such as the
// first caller's context expiring mid-command) is retried on the next call.
type cached[T any] struct {
	mu   sync.Mutex
	done bool
	val  T
}

func (c *cached[T]) get(f func() (T, error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return c.val, nil
	}
	v, err := f()
	if err != nil {
		return v, err
	}
	c.val, c.done = v, true
	return v, nil
}

type platformInfo struct{ platform, family, version string }

var (
	hostIDCache     cached[string]
	platformCache   cached[platformInfo]
	kernelVerCache  cached[string]
	kernelArchCache cached[string]
)

func HostIDWithContext(ctx context.Context) (string, error) {
	return hostIDCache.get(func() (string, error) {
		out, err := getInvoker().CommandWithContext(ctx, "uname", "-u")
		if err != nil {
			return "", err
		}
		return strings.Split(string(out), "\n")[0], nil
	})
}

// aixUtmp matches the AIX /etc/utmp binary record layout (see /usr/include/utmp.h).
// Reading utmp directly is ~180x faster than spawning `who` and avoids locale
// dependencies when parsing timestamps — ut_time is an epoch value.
type aixUtmp struct {
	User     [256]byte // ut_user: login name
	ID       [14]byte  // ut_id: inittab id
	Line     [64]byte  // ut_line: device name (pts/0, etc.)
	Pid      int32     // ut_pid
	Type     int16     // ut_type (7 = USER_PROCESS)
	Time     int64     // ut_time: epoch seconds (time64_t)
	Exit     [4]byte   // ut_exit: termination/exit status
	Host     [256]byte // ut_host: remote host
	Pad      [4]byte   // __dbl_word_pad
	Reserved [32]byte  // __reservedA[2] + __reservedV[6]
}

// UsersWithContext returns currently logged-in users by reading /etc/utmp directly.
// This avoids spawning a subprocess and eliminates locale dependencies for
// timestamp parsing — the utmp struct contains epoch seconds in ut_time.
func UsersWithContext(_ context.Context) ([]UserStat, error) {
	f, err := os.Open(utmpPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var ret []UserStat
	for {
		var entry aixUtmp
		err := binary.Read(f, binary.BigEndian, &entry)
		if err != nil {
			break // EOF or read error
		}

		// Only include active user sessions (ut_type == USER_PROCESS)
		if entry.Type != user_PROCESS {
			continue
		}

		user := strings.TrimRight(string(bytes.TrimRight(entry.User[:], "\x00")), " ")
		if user == "" {
			continue
		}

		us := UserStat{
			User:     user,
			Terminal: string(bytes.TrimRight(entry.Line[:], "\x00")),
			Host:     string(bytes.TrimRight(entry.Host[:], "\x00")),
			Started:  int(entry.Time),
		}
		ret = append(ret, us)
	}

	return ret, nil
}

// PlatformInformationWithContext returns the platform name, family, and OS
// version. These are immutable system identifiers cached after the first successful query.
func PlatformInformationWithContext(ctx context.Context) (platform, family, version string, err error) {
	info, err := platformCache.get(func() (platformInfo, error) {
		// Set the platform (which should always, and only be, "AIX") from uname(2)
		var utsname unix.Utsname
		if err := unix.Uname(&utsname); err != nil {
			return platformInfo{}, err
		}
		p := unix.ByteSliceToString(utsname.Sysname[:])

		out, err := getInvoker().CommandWithContext(ctx, "oslevel")
		if err != nil {
			return platformInfo{}, err
		}
		return platformInfo{platform: p, family: p, version: strings.TrimRight(string(out), "\n")}, nil
	})
	return info.platform, info.family, info.version, err
}

// KernelVersionWithContext returns the kernel version (e.g., "7300-03-00-2446").
// This is an immutable system identifier cached after the first successful query.
func KernelVersionWithContext(ctx context.Context) (version string, err error) {
	return kernelVerCache.get(func() (string, error) {
		out, err := getInvoker().CommandWithContext(ctx, "oslevel", "-s")
		if err != nil {
			return "", err
		}
		return strings.TrimRight(string(out), "\n"), nil
	})
}

// KernelArch returns the hardware architecture (e.g., "64").
// This is an immutable system identifier cached after the first successful query.
func KernelArch() (arch string, err error) {
	return kernelArchCache.get(func() (string, error) {
		out, err := getInvoker().Command("getconf", "KERNEL_BITMODE")
		if err != nil {
			out, err = getInvoker().Command("bootinfo", "-y")
			if err != nil {
				return "", err
			}
		}
		return strings.TrimRight(string(out), "\n"), nil
	})
}

// perfstatPartitionTotalStat reads perfstat_partition_total, swapped out in tests.
var perfstatPartitionTotalStat = common.PerfstatPartitionTotalStat

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	// Check for WPAR (Workload Partition) first — most specific virtualization layer.
	// uname -W returns "0" if not in a WPAR, or the WPAR ID if inside one.
	out, err := getInvoker().CommandWithContext(ctx, "uname", "-W")
	if err == nil {
		wparID := strings.TrimSpace(string(out))
		if wparID != "" && wparID != "0" {
			return "wpar", "guest", nil
		}
	}

	// Check for LPAR (Logical Partition) via PowerVM. Outside one, the partition
	// name is "NULL", as uname -L prints it.
	if p, err := perfstatPartitionTotalStat(); err == nil {
		if name := common.CString(p.Name[:]); name != "" && name != "NULL" {
			return "powervm", "guest", nil
		}
	}

	return "", "", nil
}
