// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package nfs

import (
	"context"
	"strconv"
	"strings"
)

// statsWithContext returns NFS statistics parsed from the nfsstat command.
func statsWithContext(ctx context.Context) (*StatsStat, error) {
	stats := &StatsStat{
		NFSClientStats: NFSClientStat{Operations: make(map[string]uint64)},
		NFSServerStats: NFSServerStat{Operations: make(map[string]uint64)},
	}

	output, err := invoke.CommandWithContext(ctx, "nfsstat")
	if err != nil {
		return nil, err
	}

	parseNfsstat(string(output), stats)
	return stats, nil
}

// ClientStatsWithContext returns NFS client statistics.
func ClientStatsWithContext(ctx context.Context) (*NFSClientStat, error) {
	stats, err := statsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &stats.NFSClientStats, nil
}

// ClientStats returns NFS client statistics.
func ClientStats() (*NFSClientStat, error) {
	return ClientStatsWithContext(context.Background())
}

// ServerStatsWithContext returns NFS server statistics.
func ServerStatsWithContext(ctx context.Context) (*NFSServerStat, error) {
	stats, err := statsWithContext(ctx)
	if err != nil {
		return nil, err
	}
	return &stats.NFSServerStats, nil
}

// ServerStats returns NFS server statistics.
func ServerStats() (*NFSServerStat, error) {
	return ServerStatsWithContext(context.Background())
}

// parseNfsstat parses `nfsstat` output on AIX.
//
// AIX organizes the output into "Server rpc:", "Server nfs:", "Client rpc:" and
// "Client nfs:" sections. Within each section a row of column NAMES is followed
// by a row of VALUES on the next line; a set of counters may span several such
// name/value line pairs. RPC sections are further split into "Connection
// oriented" (TCP) and "Connectionless" (UDP) transports, whose counters are
// summed. NFS sections report per-operation counts under "Version 2"/"Version
// 3", where an operation-name row is followed by a row of "<count> <pct>%"
// tokens.
//
// Parsing therefore remembers the most recent names row and applies the next
// values row to it, keyed by column name rather than by position so that
// multi-line counter groups and reordering are handled correctly.
func parseNfsstat(output string, stats *StatsStat) {
	var (
		section      string
		nfsVersion   string
		pendingNames []string
	)

	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			pendingNames = nil
			continue
		}

		switch {
		case strings.Contains(line, "Server rpc:"):
			section, nfsVersion, pendingNames = "server_rpc", "", nil
			continue
		case strings.Contains(line, "Server nfs:"):
			section, nfsVersion, pendingNames = "server_nfs", "", nil
			continue
		case strings.Contains(line, "Client rpc:"):
			section, nfsVersion, pendingNames = "client_rpc", "", nil
			continue
		case strings.Contains(line, "Client nfs:"):
			section, nfsVersion, pendingNames = "client_nfs", "", nil
			continue
		case strings.Contains(line, "Connection oriented"), strings.Contains(line, "Connectionless"):
			pendingNames = nil
			continue
		case strings.HasPrefix(line, "Version 2"):
			nfsVersion, pendingNames = "v2", nil
			continue
		case strings.HasPrefix(line, "Version 3"):
			nfsVersion, pendingNames = "v3", nil
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		// A names row starts with a non-numeric token; a values row is numeric.
		if !isNumericField(fields[0]) {
			pendingNames = fields
			continue
		}
		if pendingNames == nil {
			continue
		}

		if strings.Contains(line, "%") {
			applyOperationCounts(section, nfsVersion, pendingNames, fields, stats)
		} else {
			applyNamedValues(section, pendingNames, fields, stats)
		}
		pendingNames = nil
	}
}

// applyNamedValues maps a names row to its values row (1:1 by index) and
// accumulates the values into the appropriate counter struct by column name.
func applyNamedValues(section string, names, values []string, stats *StatsStat) {
	for i, name := range names {
		if i >= len(values) {
			break
		}
		v := parseUint64(values[i])
		switch section {
		case "server_rpc":
			setRPCServer(&stats.RPCServerStats, name, v)
		case "client_rpc":
			setRPCClient(&stats.RPCClientStats, name, v)
		case "server_nfs":
			setNFSServerHeader(&stats.NFSServerStats, name, v)
		case "client_nfs":
			setNFSClientHeader(&stats.NFSClientStats, name, v)
		}
	}
}

// applyOperationCounts maps an operation-name row to the following counts row.
// The counts row is a sequence of "<count> <pct>%" tokens, so the count for
// operation i is at token index 2*i. Keys are prefixed with the NFS version.
func applyOperationCounts(section, nfsVersion string, names, fields []string, stats *StatsStat) {
	var ops map[string]uint64
	switch section {
	case "server_nfs":
		ops = stats.NFSServerStats.Operations
	case "client_nfs":
		ops = stats.NFSClientStats.Operations
	default:
		return
	}

	for i, name := range names {
		countIdx := i * 2
		if countIdx >= len(fields) {
			break
		}
		key := name
		if nfsVersion != "" {
			key = nfsVersion + "_" + name
		}
		ops[key] = parseUint64(fields[countIdx])
	}
}

func setRPCServer(s *RPCServerStat, name string, v uint64) {
	switch name {
	case "calls":
		s.Calls += v
	case "badcalls":
		s.BadCalls += v
	case "nullrecv":
		s.NullRecv += v
	case "badlen":
		s.BadLen += v
	case "xdrcall":
		s.XdrCall += v
	case "dupchecks":
		s.DupChecks += v
	case "dupreqs":
		s.DupReqs += v
	}
}

func setRPCClient(s *RPCClientStat, name string, v uint64) {
	switch name {
	case "calls":
		s.Calls += v
	case "badcalls":
		s.BadCalls += v
	case "badxids":
		s.BadXIDs += v
	case "timeouts":
		s.Timeouts += v
	case "newcreds":
		s.NewCreds += v
	case "badverfs":
		s.BadVerfs += v
	case "timers":
		s.Timers += v
	case "nomem":
		s.NoMem += v
	case "cantconn":
		s.CantConn += v
	case "interrupts":
		s.Interrupts += v
	case "retrans":
		s.Retrans += v
	case "cantsend":
		s.CantSend += v
	}
}

func setNFSServerHeader(s *NFSServerStat, name string, v uint64) {
	switch name {
	case "calls":
		s.Calls += v
	case "badcalls":
		s.BadCalls += v
	case "public_v2":
		s.PublicV2 += v
	case "public_v3":
		s.PublicV3 += v
	}
}

func setNFSClientHeader(s *NFSClientStat, name string, v uint64) {
	switch name {
	case "calls":
		s.Calls += v
	case "badcalls":
		s.BadCalls += v
	case "clgets":
		s.ClGets += v
	case "cltoomany":
		s.ClTooMany += v
	}
}

// isNumericField reports whether a token is a numeric value (optionally with a
// trailing percent sign), distinguishing values rows from names rows.
func isNumericField(s string) bool {
	_, err := strconv.ParseUint(strings.TrimSuffix(s, "%"), 10, 64)
	return err == nil
}

// parseUint64 parses a string to uint64, returning 0 on error.
func parseUint64(s string) uint64 {
	val, _ := strconv.ParseUint(strings.TrimSpace(strings.TrimSuffix(s, "%")), 10, 64)
	return val
}
