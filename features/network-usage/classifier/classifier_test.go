package classifier

import (
	"net/netip"
	"testing"
)

func TestLANConnectionDoesNotCountAsInternet(t *testing.T) {
	got := ClassifyRemoteIP(netip.MustParseAddr("192.168.1.3"), DefaultConfig())

	if got.CountsAsInternet {
		t.Fatalf("LAN traffic was counted as Internet: %+v", got)
	}
	if got.Class != TrafficLAN {
		t.Fatalf("expected LAN classification, got %s", got.Class)
	}
}

func TestPublicConnectionCountsAsInternet(t *testing.T) {
	got := ClassifyRemoteIP(netip.MustParseAddr("8.8.8.8"), DefaultConfig())

	if !got.CountsAsInternet {
		t.Fatalf("public traffic was not counted as Internet: %+v", got)
	}
	if got.Class != TrafficInternet {
		t.Fatalf("expected Internet classification, got %s", got.Class)
	}
}

func TestDockerInternalConnectionDoesNotCountAsInternet(t *testing.T) {
	got := ClassifyRemoteIP(netip.MustParseAddr("172.20.0.5"), DefaultConfig())

	if got.CountsAsInternet {
		t.Fatalf("Docker internal traffic was counted as Internet: %+v", got)
	}
	if got.Class != TrafficDocker {
		t.Fatalf("expected Docker internal classification, got %s", got.Class)
	}
}

func TestLoopbackConnectionDoesNotCountAsInternet(t *testing.T) {
	got := ClassifyRemoteIP(netip.MustParseAddr("127.0.0.1"), DefaultConfig())

	if got.CountsAsInternet {
		t.Fatalf("loopback traffic was counted as Internet: %+v", got)
	}
	if got.Class != TrafficLoopback {
		t.Fatalf("expected loopback classification, got %s", got.Class)
	}
}
