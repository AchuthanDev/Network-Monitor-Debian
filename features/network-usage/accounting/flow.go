package accounting

import (
	"net/netip"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/conntrack"
)

type TrafficDelta struct {
	ObservedAt            time.Time
	Protocol              string
	LocalIP               netip.Addr
	LocalPort             uint16
	RemoteIP              netip.Addr
	RemotePort            uint16
	Class                 classifier.TrafficClass
	DownloadBytes         uint64
	UploadBytes           uint64
	AttributionSource     string
	AttributionConfidence string
	FlowKey               string
}

type LocalMatcher struct {
	HostIPs    map[netip.Addr]struct{}
	LocalCIDRs []netip.Prefix
}

func NewLocalMatcher(hostIPs []netip.Addr, localCIDRs []netip.Prefix) LocalMatcher {
	hostMap := make(map[netip.Addr]struct{}, len(hostIPs))
	for _, ip := range hostIPs {
		if ip.IsValid() {
			hostMap[ip] = struct{}{}
		}
	}
	return LocalMatcher{HostIPs: hostMap, LocalCIDRs: localCIDRs}
}

func (m LocalMatcher) IsLocal(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}
	if _, ok := m.HostIPs[addr]; ok {
		return true
	}
	for _, prefix := range m.LocalCIDRs {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

type Direction struct {
	LocalIP     netip.Addr
	LocalPort   uint16
	RemoteIP    netip.Addr
	RemotePort  uint16
	DownloadKey string
	UploadKey   string
}

func ResolveDirection(flow conntrack.Flow, matcher LocalMatcher) (Direction, bool) {
	origSrcLocal := matcher.IsLocal(flow.Original.SrcIP)
	origDstLocal := matcher.IsLocal(flow.Original.DstIP)

	if origSrcLocal && !origDstLocal {
		return Direction{
			LocalIP:     flow.Original.SrcIP,
			LocalPort:   flow.Original.SrcPort,
			RemoteIP:    flow.Original.DstIP,
			RemotePort:  flow.Original.DstPort,
			UploadKey:   "original",
			DownloadKey: "reply",
		}, true
	}

	if origDstLocal && !origSrcLocal {
		return Direction{
			LocalIP:     flow.Original.DstIP,
			LocalPort:   flow.Original.DstPort,
			RemoteIP:    flow.Original.SrcIP,
			RemotePort:  flow.Original.SrcPort,
			UploadKey:   "reply",
			DownloadKey: "original",
		}, true
	}

	return Direction{}, false
}

func BuildDelta(now time.Time, flow conntrack.Flow, prev *conntrack.Counters, matcher LocalMatcher, cfg classifier.Config) (TrafficDelta, bool) {
	direction, ok := ResolveDirection(flow, matcher)
	if !ok || prev == nil {
		return TrafficDelta{}, false
	}

	origDelta := positiveDelta(flow.Counters.OriginalBytes, prev.OriginalBytes)
	replyDelta := positiveDelta(flow.Counters.ReplyBytes, prev.ReplyBytes)
	if origDelta == 0 && replyDelta == 0 {
		return TrafficDelta{}, false
	}

	var download uint64
	var upload uint64
	if direction.DownloadKey == "original" {
		download = origDelta
		upload = replyDelta
	} else {
		download = replyDelta
		upload = origDelta
	}

	classification := classifier.ClassifyRemoteIP(direction.RemoteIP, cfg)
	return TrafficDelta{
		ObservedAt:            now.UTC(),
		Protocol:              flow.Protocol,
		LocalIP:               direction.LocalIP,
		LocalPort:             direction.LocalPort,
		RemoteIP:              direction.RemoteIP,
		RemotePort:            direction.RemotePort,
		Class:                 classification.Class,
		DownloadBytes:         download,
		UploadBytes:           upload,
		AttributionSource:     "conntrack",
		AttributionConfidence: "host_flow",
		FlowKey:               flow.Key(),
	}, true
}

func positiveDelta(current uint64, previous uint64) uint64 {
	if current <= previous {
		return 0
	}
	return current - previous
}
