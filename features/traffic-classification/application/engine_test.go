package application

import (
	"net/netip"
	"testing"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/infrastructure/providers"
)

func TestClassifyYouTubeFromDNSEvidence(t *testing.T) {
	engine := NewEngine(toProviders(providers.DefaultProviders())...)
	flow := domain.Flow{
		Timestamp:       time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC),
		SourceIP:        netip.MustParseAddr("192.168.50.21"),
		DestinationIP:   netip.MustParseAddr("142.250.190.14"),
		Protocol:        "tcp",
		DestinationPort: 443,
		DNSEvidence: []domain.DNSEvidence{{
			Query:      "rr1---sn.googlevideo.com",
			ResolvedIP: netip.MustParseAddr("142.250.190.14"),
			ObservedAt: time.Date(2026, 8, 15, 13, 59, 58, 0, time.UTC),
			Source:     "pihole",
		}},
	}
	got := engine.Classify(flow)
	if got.Service != "YouTube" || got.Category != domain.CategoryVideoStreaming || got.Confidence != domain.ConfidenceHigh {
		t.Fatalf("unexpected classification: %+v", got)
	}
}

func TestUnknownHTTPSIsNotOverClassified(t *testing.T) {
	engine := NewEngine(toProviders(providers.DefaultProviders())...)
	got := engine.Classify(domain.Flow{
		DestinationIP:   netip.MustParseAddr("203.0.113.50"),
		Protocol:        "tcp",
		DestinationPort: 443,
	})
	if got.Category != domain.CategoryUnknownHTTPS || got.Confidence != domain.ConfidenceUnknown {
		t.Fatalf("expected unknown HTTPS, got %+v", got)
	}
}

func TestGenericQUICUsesLowConfidenceProtocolOnly(t *testing.T) {
	engine := NewEngine()
	got := engine.Classify(domain.Flow{Protocol: "udp", DestinationPort: 443})
	if got.Category != domain.CategoryQUIC || got.Confidence != domain.ConfidenceLow {
		t.Fatalf("expected low confidence QUIC, got %+v", got)
	}
}

func toProviders(items []providers.DomainProvider) []Provider {
	result := make([]Provider, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}
