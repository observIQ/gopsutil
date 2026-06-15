// SPDX-License-Identifier: BSD-3-Clause
//go:build aix

package net

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
