package dns

import (
	"net/netip"
	"testing"
	"time"
)

func TestCorrelatorMatchesClientDestinationAndWindow(t *testing.T) {
	now := time.Date(2026, 8, 15, 14, 5, 0, 0, time.UTC)
	c := NewCorrelator(5 * time.Minute)
	c.Add(Query{
		ClientIP:   netip.MustParseAddr("192.168.50.21"),
		Domain:     "rr1---sn.googlevideo.com",
		ResolvedIP: netip.MustParseAddr("142.250.190.14"),
		ObservedAt: now.Add(-30 * time.Second),
		Source:     "pihole",
	})
	c.Add(Query{
		ClientIP:   netip.MustParseAddr("192.168.50.22"),
		Domain:     "netflix.com",
		ResolvedIP: netip.MustParseAddr("142.250.190.14"),
		ObservedAt: now.Add(-30 * time.Second),
		Source:     "pihole",
	})

	evidence := c.Evidence(netip.MustParseAddr("192.168.50.21"), netip.MustParseAddr("142.250.190.14"), now)
	if len(evidence) != 1 || evidence[0].Query != "rr1---sn.googlevideo.com" {
		t.Fatalf("unexpected evidence: %+v", evidence)
	}
}
