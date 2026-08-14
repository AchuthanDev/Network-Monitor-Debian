package accounting

import (
	"net/netip"
	"testing"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/conntrack"
)

func TestOutboundPublicFlowCountsUploadAndDownload(t *testing.T) {
	flow := testFlow("192.168.1.10", "8.8.8.8", 1000, 5000)
	prev := &conntrack.Counters{OriginalBytes: 800, ReplyBytes: 3000}
	matcher := NewLocalMatcher([]netip.Addr{netip.MustParseAddr("192.168.1.10")}, nil)

	delta, ok := BuildDelta(time.Unix(0, 0), flow, prev, matcher, classifier.DefaultConfig())
	if !ok {
		t.Fatal("expected delta")
	}
	if delta.Class != classifier.TrafficInternet {
		t.Fatalf("expected internet, got %s", delta.Class)
	}
	if delta.UploadBytes != 200 || delta.DownloadBytes != 2000 {
		t.Fatalf("unexpected delta download=%d upload=%d", delta.DownloadBytes, delta.UploadBytes)
	}
}

func TestOutboundLANFlowDoesNotCountAsInternet(t *testing.T) {
	flow := testFlow("192.168.1.10", "192.168.1.3", 1000, 5000)
	prev := &conntrack.Counters{OriginalBytes: 0, ReplyBytes: 0}
	matcher := NewLocalMatcher([]netip.Addr{netip.MustParseAddr("192.168.1.10")}, nil)

	delta, ok := BuildDelta(time.Unix(0, 0), flow, prev, matcher, classifier.DefaultConfig())
	if !ok {
		t.Fatal("expected delta")
	}
	if delta.Class != classifier.TrafficLAN {
		t.Fatalf("expected lan, got %s", delta.Class)
	}
}

func TestFirstSnapshotIsBaseline(t *testing.T) {
	flow := testFlow("192.168.1.10", "8.8.8.8", 1000, 5000)
	matcher := NewLocalMatcher([]netip.Addr{netip.MustParseAddr("192.168.1.10")}, nil)

	_, ok := BuildDelta(time.Unix(0, 0), flow, nil, matcher, classifier.DefaultConfig())
	if ok {
		t.Fatal("first snapshot must not emit historical bytes")
	}
}

func testFlow(src string, dst string, origBytes uint64, replyBytes uint64) conntrack.Flow {
	return conntrack.Flow{
		Family:   "ipv4",
		Protocol: "tcp",
		Original: conntrack.Tuple{
			SrcIP:   netip.MustParseAddr(src),
			DstIP:   netip.MustParseAddr(dst),
			SrcPort: 12345,
			DstPort: 443,
		},
		Reply: conntrack.Tuple{
			SrcIP:   netip.MustParseAddr(dst),
			DstIP:   netip.MustParseAddr(src),
			SrcPort: 443,
			DstPort: 12345,
		},
		Counters: conntrack.Counters{
			OriginalBytes: origBytes,
			ReplyBytes:    replyBytes,
		},
	}
}
