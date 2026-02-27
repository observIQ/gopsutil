// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package disk

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v4/internal/common"
)

func IOCountersWithContext(ctx context.Context, names ...string) (map[string]IOCountersStat, error) {
	out, err := invoke.CommandWithContext(ctx, "iostat", "-d")
	if err != nil {
		return nil, err
	}

	ret := make(map[string]IOCountersStat)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		// Skip the header line
		if fields[0] == "Disks:" {
			continue
		}

		name := fields[0]
		if len(names) > 0 && !common.StringsHas(names, name) {
			continue
		}

		kbRead, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			continue
		}
		kbWritten, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			continue
		}

		ret[name] = IOCountersStat{
			Name:       name,
			ReadBytes:  kbRead * 1024,
			WriteBytes: kbWritten * 1024,
		}
	}

	return ret, nil
}

func LabelWithContext(_ context.Context, _ string) (string, error) {
	return "", common.ErrNotImplementedError
}

// Using lscfg and a device name, we can get the device information
// This is a pure go implementation, and should be moved to disk_aix_nocgo.go
// if a more efficient CGO method is introduced in disk_aix_cgo.go
func SerialNumberWithContext(ctx context.Context, name string) (string, error) {
	// This isn't linux, these aren't actual disk devices
	if strings.HasPrefix(name, "/dev/") {
		return "", errors.New("devices on /dev are not physical disks on aix")
	}
	out, err := invoke.CommandWithContext(ctx, "lscfg", "-vl", name)
	if err != nil {
		return "", err
	}

	ret := ""
	// Kind of inefficient, but it works
	lines := strings.Split(string(out), "\n")
	for line := 1; line < len(lines); line++ {
		v := strings.TrimSpace(lines[line])
		if strings.HasPrefix(v, "Serial Number...............") {
			ret = strings.TrimPrefix(v, "Serial Number...............")
			if ret == "" {
				return "", errors.New("empty serial for disk")
			}
			return ret, nil
		}
	}

	return ret, errors.New("serial entry not found for disk")
}
