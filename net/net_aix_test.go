// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package net

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/shirou/gopsutil/v4/internal/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample netstat -Aan output captured from AIX 7.3 for unit testing.
const testNetstatAanInet = `Active Internet connections (including servers)
PCB/ADDR         Proto Recv-Q Send-Q  Local Address      Foreign Address    (state)
f1000f00055bcbc0 tcp        0      0  *.*                   *.*                   CLOSED
f1000f00055963c0 tcp4       0      0  *.*                   *.*                   CLOSED
f1000f000017e3c0 tcp6       0      0  *.22                  *.*                   LISTEN
f1000f0000287bc0 tcp4       0      0  *.22                  *.*                   LISTEN
f1000f000017f3c0 tcp4       0      0  *.25                  *.*                   LISTEN
f1000f00002d8bc0 tcp4       0      0  192.168.242.122.22    24.236.207.124.33236  ESTABLISHED
f1000f00055eabc0 tcp6       0      0  ::1.6010              *.*                   LISTEN
f1000f000561c3c0 tcp4       0      0  192.168.242.122.34898 34.120.255.184.443    ESTABLISHED
f1000f0000140600 udp        0      0  *.111                 *.*
f1000f000017be00 udp        0      0  *.161                 *.*
f1000f0005582e00 udp        0      0  *.514                 *.*
`

const testNetstatAanUnix = `Active UNIX domain sockets
SADR/PCB         Type   Recv-Q Send-Q      Inode            Conn             Refs           Nextref      Addr
f1000f0000144808 dgram       0      0                0 f1000f00055a1b00                0                0
f1000f0000145580
f1000f000010a408 dgram       0      0 f1000b02a0354c20                0                0                0 /dev/SRC
f1000f000557b280
f1000f0000169808 stream      0      0 f1000b02a03a9420                0                0                0 /var/ct/IW/soc/mc/RMIBM.DRM.0
f1000f0000177780
f1000f000015c808 stream      0      0                0 f1000f0000177580                0                0
f1000f0000177880
`

const testNetstatAanFull = testNetstatAanInet + "\n" + testNetstatAanUnix

func TestParseNetstatAanInet(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanInet, "inet")
	require.NoError(t, err)

	// Should skip *.*  local addresses (2 CLOSED entries) and include the rest
	// Expected: *.22 tcp6, *.22 tcp4, *.25 tcp4, ESTABLISHED ssh, ::1.6010, ESTABLISHED https,
	//           *.111 udp, *.161 udp, *.514 udp = 9 entries
	assert.Len(t, entries, 9)

	// Verify the ESTABLISHED TCP connection
	var sshConn *netstatAanEntry
	for i := range entries {
		if entries[i].conn.Status == "ESTABLISHED" && entries[i].conn.Laddr.Port == 22 {
			sshConn = &entries[i]
			break
		}
	}
	require.NotNil(t, sshConn, "should find the SSH ESTABLISHED connection")
	assert.Equal(t, "f1000f00002d8bc0", sshConn.sockAddr)
	assert.Equal(t, "tcp4", sshConn.proto)
	assert.Equal(t, uint32(syscall.AF_INET), sshConn.conn.Family)
	assert.Equal(t, uint32(syscall.SOCK_STREAM), sshConn.conn.Type)
	assert.Equal(t, "192.168.242.122", sshConn.conn.Laddr.IP)
	assert.Equal(t, uint32(22), sshConn.conn.Laddr.Port)
	assert.Equal(t, "24.236.207.124", sshConn.conn.Raddr.IP)
	assert.Equal(t, uint32(33236), sshConn.conn.Raddr.Port)

	// Verify a LISTEN entry
	var listenConn *netstatAanEntry
	for i := range entries {
		if entries[i].conn.Status == "LISTEN" && entries[i].conn.Laddr.Port == 25 {
			listenConn = &entries[i]
			break
		}
	}
	require.NotNil(t, listenConn, "should find the SMTP LISTEN connection")
	assert.Equal(t, "0.0.0.0", listenConn.conn.Laddr.IP)
	assert.Equal(t, uint32(25), listenConn.conn.Laddr.Port)

	// Verify IPv6 LISTEN entry
	var ipv6Conn *netstatAanEntry
	for i := range entries {
		if entries[i].proto == "tcp6" && entries[i].conn.Laddr.Port == 22 {
			ipv6Conn = &entries[i]
			break
		}
	}
	require.NotNil(t, ipv6Conn, "should find the IPv6 SSH LISTEN connection")
	assert.Equal(t, uint32(syscall.AF_INET6), ipv6Conn.conn.Family)
	assert.Equal(t, "::", ipv6Conn.conn.Laddr.IP)

	// Verify UDP entry (no state field)
	var udpConn *netstatAanEntry
	for i := range entries {
		if entries[i].conn.Laddr.Port == 161 {
			udpConn = &entries[i]
			break
		}
	}
	require.NotNil(t, udpConn, "should find the SNMP UDP connection")
	assert.Equal(t, uint32(syscall.SOCK_DGRAM), udpConn.conn.Type)
	assert.Empty(t, udpConn.conn.Status)
}

func TestParseNetstatAanTCPFilter(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanInet, "tcp")
	require.NoError(t, err)

	for _, entry := range entries {
		assert.True(t, strings.HasPrefix(entry.proto, "tcp"),
			"tcp filter should only return TCP entries, got proto=%s", entry.proto)
	}
}

func TestParseNetstatAanUDPFilter(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanInet, "udp")
	require.NoError(t, err)

	assert.Len(t, entries, 3)
	for _, entry := range entries {
		assert.True(t, strings.HasPrefix(entry.proto, "udp"),
			"udp filter should only return UDP entries, got proto=%s", entry.proto)
	}
}

func TestParseNetstatAanUnix(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanUnix, "unix")
	require.NoError(t, err)

	// 4 unix socket entries (each spans 2 lines; second line is skipped)
	assert.Len(t, entries, 4)

	// Verify dgram entry with address
	var srcConn *netstatAanEntry
	for i := range entries {
		if entries[i].conn.Laddr.IP == "/dev/SRC" {
			srcConn = &entries[i]
			break
		}
	}
	require.NotNil(t, srcConn, "should find the /dev/SRC unix socket")
	assert.Equal(t, uint32(syscall.AF_UNIX), srcConn.conn.Family)
	assert.Equal(t, uint32(syscall.SOCK_DGRAM), srcConn.conn.Type)

	// Verify stream entry
	var streamConn *netstatAanEntry
	for i := range entries {
		if entries[i].conn.Type == uint32(syscall.SOCK_STREAM) {
			streamConn = &entries[i]
			break
		}
	}
	require.NotNil(t, streamConn, "should find a stream unix socket")
	assert.Equal(t, uint32(syscall.AF_UNIX), streamConn.conn.Family)
}

func TestParseNetstatAanAll(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanFull, "all")
	require.NoError(t, err)

	// Should include both inet and unix entries
	var hasInet, hasUnix bool
	for _, entry := range entries {
		if entry.proto == "unix" {
			hasUnix = true
		} else {
			hasInet = true
		}
	}
	assert.True(t, hasInet, "should have inet entries")
	assert.True(t, hasUnix, "should have unix entries")
}

func TestParseNetstatAanUnixFilterExcludesInet(t *testing.T) {
	entries, err := parseNetstatAan(testNetstatAanFull, "unix")
	require.NoError(t, err)

	for _, entry := range entries {
		assert.Equal(t, "unix", entry.proto,
			"unix filter should exclude inet entries")
	}
}

func TestParseAIXRmsockPid(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int32
	}{
		{
			// AIX has a known typo: "proc" + "cess" (double c)
			name:   "AIX typo proc" + "cess",
			output: "The socket 0xf1000f00002d8808 is being held by proc" + "cess 21496092 (sshd).",
			want:   21496092,
		},
		{
			// Handle correct spelling too in case a future AIX version fixes it
			name:   "correct spelling process",
			output: "The socket 0xf1000f00002d8808 is being held by process 14287304 (sshd).",
			want:   14287304,
		},
		{
			name:   "wait for exiting processes",
			output: "Wait for exiting processes to be cleaned up before removing the socket",
			want:   0,
		},
		{
			name:   "not a socket",
			output: "It is not a socket",
			want:   0,
		},
		{
			name:   "kernel address error",
			output: "rmsock : Unable to read kernel address f1000f0000140600, errno = 22",
			want:   0,
		},
		{
			name:   "empty output",
			output: "",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAIXRmsockPid(tt.output)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Integration tests below require running on AIX with real netstat/rmsock.

func TestConnectionsPidWithContext(t *testing.T) {
	ctx := context.Background()
	pid := int32(os.Getpid())

	conns, err := ConnectionsPidWithContext(ctx, "inet", pid)
	if errors.Is(err, common.ErrNotImplementedError) {
		t.Skip("not implemented")
	}
	require.NoError(t, err)
	if conns != nil {
		assert.IsType(t, []ConnectionStat{}, conns)
		for _, conn := range conns {
			assert.NotEmpty(t, conn.Family)
			assert.NotEmpty(t, conn.Type)
		}
	}
}

func TestConnectionsPidWithContextAll(t *testing.T) {
	ctx := context.Background()
	pid := int32(os.Getpid())

	conns, err := ConnectionsPidWithContext(ctx, "all", pid)
	if errors.Is(err, common.ErrNotImplementedError) {
		t.Skip("not implemented")
	}
	require.NoError(t, err)
	if conns != nil {
		assert.IsType(t, []ConnectionStat{}, conns)
	}
}

func TestConnectionsPidWithContextUDP(t *testing.T) {
	ctx := context.Background()
	pid := int32(os.Getpid())

	conns, err := ConnectionsPidWithContext(ctx, "udp", pid)
	if errors.Is(err, common.ErrNotImplementedError) {
		t.Skip("not implemented")
	}
	require.NoError(t, err)
	if conns != nil {
		assert.IsType(t, []ConnectionStat{}, conns)
	}
}

func TestConnectionsWithContext(t *testing.T) {
	ctx := context.Background()

	conns, err := ConnectionsWithContext(ctx, "inet")
	require.NoError(t, err)
	assert.NotNil(t, conns)
	assert.IsType(t, []ConnectionStat{}, conns)
	assert.NotEmpty(t, conns)

	for _, conn := range conns {
		assert.NotEmpty(t, conn.Family)
		assert.NotEmpty(t, conn.Type)
	}
}

// Abbreviated netstat -s fixture from AIX 7.3 for unit testing.
const testNetstatS = `icmp:
	3 calls to icmp_error
	0 errors not generated because old message was icmp
	Output histogram:
		destination unreachable: 3
	12 messages with bad code fields
	0 messages < minimum length
	0 bad checksums
	0 messages with bad length
	Input histogram:
		destination unreachable: 24
		time exceeded: 34
	0 message responses generated
tcp:
	8864927 packets sent
		4194757 data packets (3255534866 bytes)
		26849 data packets (23271573 bytes) retransmitted
		3174391 ack-only packets (1213424 delayed)
	13907119 packets received
		4921506 acks (for 3255334387 bytes)
		1443623 duplicate acks
	2177 connection requests
	465222 connection accepts
	467064 connections established (including accepts)
	467502 connections closed (including 1913 drops)
udp:
	1886908 datagrams received
	0 incomplete headers
	0 bad checksums
	222742 dropped due to no socket
	1481568 delivered
	2224137 datagrams output
ip:
	16177190 total packets received
	0 bad header checksums
	15571192 packets for this host
	58 packets for unknown/unsupported protocol
	0 packets forwarded
	11912013 packets sent from this host
`

func TestParseNetstatS(t *testing.T) {
	data, err := os.ReadFile("testdata/aix/netstat_s.txt")
	require.NoError(t, err)

	stats, err := parseNetstatS(string(data), nil)
	require.NoError(t, err)

	protos := map[string]ProtoCountersStat{}
	for _, s := range stats {
		protos[s.Protocol] = s
	}

	check := func(t *testing.T, proto string, want map[string]int64) {
		t.Helper()
		s, ok := protos[proto]
		require.True(t, ok, "%s section not found", proto)
		assert.Equal(t, want, s.Stats)
	}

	// Superset contract: MIB-II/RFC-1213 keys (PascalCase) where a counter maps,
	// plus AIX-native counters (camelCase) for everything else `netstat -s`
	// reports. This is an exact match of the full output, including the icmp and
	// igmp sections that a MIB-II-only mapping would drop entirely.
	var gotProtocols []string
	for p := range protos {
		gotProtocols = append(gotProtocols, p)
	}
	assert.ElementsMatch(t, []string{"icmp", "igmp", "tcp", "udp", "ip", "ipv6"}, gotProtocols)

	t.Run("icmp", func(t *testing.T) {
		check(t, "icmp", map[string]int64{
			"badChecksums":     0,
			"callsToIcmpError": 0,
			"errorsNotGeneratedBecauseOldMessageWasIcmp": 0,
			"messageResponsesGenerated":                  0,
			"messagesMinimumLength":                      0,
			"messagesWithBadCodeFields":                  0,
			"messagesWithBadLength":                      0,
		})
	})

	t.Run("igmp", func(t *testing.T) {
		check(t, "igmp", map[string]int64{
			"membershipQueriesReceived":                         0,
			"membershipQueriesReceivedWithInvalidField":         0,
			"membershipReportsReceived":                         0,
			"membershipReportsReceivedForGroupsToWhichWeBelong": 0,
			"membershipReportsReceivedWithInvalidField":         0,
			"membershipReportsSent":                             0,
			"messagesReceived":                                  0,
			"messagesReceivedWithBadChecksum":                   0,
			"messagesReceivedWithTooFewBytes":                   0,
		})
	})

	t.Run("tcp", func(t *testing.T) {
		check(t, "tcp", map[string]int64{
			// MIB-II keys (PascalCase). InErrs/etc. aggregate several AIX lines.
			"ActiveOpens":  404,
			"AttemptFails": 606,
			"InCsumErrors": 707,
			"InErrs":       1603, // 801 (bad header offset) + 802 (packet too short)
			"InSegs":       2002,
			"OutSegs":      1001,
			"PassiveOpens": 505,
			"RetransSegs":  303,
			// Native counters (camelCase).
			"ackOnlyPackets":                               100,
			"ackPacketHeadersCorrectlyPredicted":           600,
			"acks":                                         700,
			"acksForUnsentData":                            0,
			"bytesIsTheBiggestLargesend":                   0,
			"bytesSentUsingLargesend":                      0,
			"completelyDuplicatePackets":                   0,
			"connectionsClosed":                            100,
			"connectionsDroppedByKeepalive":                0,
			"connectionsDroppedByRexmitTimeout":            0,
			"connectionsDroppedDueToBadAcks":               0,
			"connectionsDroppedDueToDuplicateSynPackets":   0,
			"connectionsDroppedDueToMaxAssemblyQueueDepth": 0,
			"connectionsDroppedDueToPersistTimeout":        0,
			"connectionsEstablished":                       600,
			"connectionsInTimewaitReused":                  0,
			"connectionsWithEcnCapability":                 0,
			"controlPackets":                               2,
			"dataInjectionSegmentsDropped":                 0,
			"dataPacketHeadersCorrectlyPredicted":          900,
			"dataPackets":                                  800,
			"delayedAcksForFin":                            0,
			"delayedAcksForSyn":                            0,
			"discardedByListeners":                         5,
			"discardedDueToListenerSQueueFull":             0,
			"duplicateAcks":                                0,
			"exhaustedEphemeralPortsErrors":                0,
			"fakeRstSegmentsDropped":                       0,
			"fakeSynSegmentsDropped":                       0,
			"fastRetransmits":                              0,
			"fastpathLoopbackConnections":                  0,
			"fastpathLoopbackReceivedPackets":              0,
			"fastpathLoopbackSentPackets":                  0,
			"keepaliveProbesSent":                          0,
			"keepaliveTimeouts":                            0,
			"largeSends":                                   0,
			"newrenoRetransmits":                           0,
			"oldDuplicatePackets":                          0,
			"outOfOrderPackets":                            0,
			"packetsDroppedDueToMemoryAllocationFailure":   0,
			"packetsOfDataAfterWindow":                     0,
			"packetsReceivedAfterClose":                    0,
			"packetsReceivedInSequence":                    1200,
			"packetsWithBadHardwareAssistedChecksum":       0,
			"packetsWithSomeDupData":                       0,
			"pathMtuDiscoveryTerminationsDueToRetransmits": 0,
			"persistTimeouts":                              0,
			"resendsDueToPathMtuDiscovery":                 0,
			"retransmitTimeouts":                           0,
			"segmentsUpdatedRtt":                           500,
			"segmentsWithCongestionExperiencedBitSet":      0,
			"segmentsWithCongestionWindowReducedBitSet":    0,
			"sendAndDisconnects":                           0,
			"splicedConnections":                           0,
			"splicedConnectionsClosed":                     0,
			"splicedConnectionsKeepaliveTimeout":           0,
			"splicedConnectionsPersistTimeout":             0,
			"splicedConnectionsReset":                      0,
			"splicedConnectionsTimeout":                    0,
			"tcpChecksumOffloadDisabledDuringRetransmit":   0,
			"tcptrConnectionsDroppedForNoMemory":           0,
			"tcptrMaxConnectionsDropped":                   0,
			"tcptrMaximumPerHostConnectionsDropped":        0,
			"timesAvoidedFalseFastRetransmits":             0,
			"timesRespondedToEcn":                          0,
			"timesSackBlocksArrayIsExtended":               0,
			"timesSackHolesArrayIsExtended":                0,
			"urgOnlyPackets":                               0,
			"whenCongestionWindowLessThan4Segments":        0,
			"windowProbePackets":                           0,
			"windowProbes":                                 0,
			"windowUpdatePackets":                          0,
		})
	})

	t.Run("udp", func(t *testing.T) {
		check(t, "udp", map[string]int64{
			"InCsumErrors":                  5500,
			"InDatagrams":                   1100,
			"InErrors":                      330,  // 110 (incomplete headers) + 220 (bad data length)
			"NoPorts":                       7700, // 3300 (unicast) + 4400 (broadcast)
			"OutDatagrams":                  2200,
			"RcvbufErrors":                  6600,
			"datagramsReceived":             9999,
			"exhaustedEphemeralPortsErrors": 0,
		})
	})

	t.Run("ip", func(t *testing.T) {
		check(t, "ip", map[string]int64{
			"ForwDatagrams":                         100,
			"FragCreates":                           600,
			"InDelivers":                            40000,
			"InDiscards":                            500,
			"InHdrErrors":                           308, // 11+22+33+44+55+66+77 (seven header-error lines)
			"InReceives":                            50000,
			"InUnknownProtos":                       9000,
			"OutDiscards":                           800,
			"OutNoRoutes":                           700,
			"OutRequests":                           30000,
			"ReasmFails":                            300,
			"ReasmOKs":                              200,
			"ReasmReqds":                            900,
			"datagramsThatCanTBeFragmented":         0,
			"incomingPacketsDroppedDueToMlsFilters": 0,
			"ipMulticastPacketsDroppedDueToNoReceiver":      0,
			"ipintrqOverflows":                              0,
			"outputDatagramsFragmented":                     0,
			"packetsDroppedByThreads":                       0,
			"packetsDroppedDueToTheFullSocketReceiveBuffer": 0,
			"packetsNotForwardable":                         1234,
			"packetsNotSentDueToMlsFilters":                 0,
			"packetsProcessedByThreads":                     0,
			"packetsSentWithFabricatedIpHeader":             0,
			"redirectsSent":                                 0,
			"withIllegalSource":                             0,
		})
	})

	t.Run("ipv6", func(t *testing.T) {
		check(t, "ipv6", map[string]int64{
			"ForwDatagrams":                         110,
			"FragCreates":                           660,
			"InDelivers":                            55000,
			"InDiscards":                            651, // 550 (fragments) + 101 (input no memory)
			"InHdrErrors":                           110, // 11+22+33+44 (four header-error lines)
			"InReceives":                            60000,
			"InUnknownProtos":                       880,
			"OutDiscards":                           777, // 333 (no bufs) + 444 (no memory)
			"OutNoRoutes":                           770,
			"OutRequests":                           45000,
			"ReasmFails":                            330,
			"ReasmOKs":                              220,
			"ReasmReqds":                            990,
			"incomingPacketsDroppedDueToMlsFilters": 0,
			"outputDatagramsFragmented":             0,
			"packetsDroppedDueToTheFullSocketReceiveBuffer": 0,
			"packetsNotDeliveredDueToBadRawIpv6Checksum":    0,
			"packetsNotForwardable":                         0,
			"packetsNotSentDueToMlsFilters":                 0,
			"packetsSentWithFabricatedIpv6Header":           0,
			"tooBigPacketsNotForwarded":                     0,
		})
	})
}

func TestParseNetstatSFiltered(t *testing.T) {
	data, err := os.ReadFile("testdata/aix/netstat_s.txt")
	require.NoError(t, err)

	stats, err := parseNetstatS(string(data), []string{"tcp", "udp"})
	require.NoError(t, err)

	protos := map[string]ProtoCountersStat{}
	for _, s := range stats {
		protos[s.Protocol] = s
	}
	assert.Contains(t, protos, "tcp")
	assert.Contains(t, protos, "udp")
	assert.NotContains(t, protos, "ip")
	assert.NotContains(t, protos, "ipv6")
}

func TestNormaliseNetstatDesc(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"packets sent", "packets sent"},
		{"data packets (6302116893 bytes) retransmitted", "data packets retransmitted"},
		{"823508 segments updated rtt (of 120622 attempts)", "823508 segments updated rtt"},
		{"connections closed (including 74 drops)", "connections closed"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, normaliseNetstatDesc(tc.input), "input: %q", tc.input)
	}
}
