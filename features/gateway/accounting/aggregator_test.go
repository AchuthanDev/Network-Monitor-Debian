package accounting

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/devices"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/isp"
)

func TestMemoryAggregatorSeparatesDownloadUploadAndISPPeriod(t *testing.T) {
	agg := NewMemoryAggregator(isp.DefaultWindowConfig())
	device := devices.Device{ID: "dev_phone"}
	base := time.Date(2026, 8, 15, 1, 30, 0, 0, mustLoadLocation(t, "Asia/Colombo"))

	for _, event := range []TrafficEvent{
		{Timestamp: base, Direction: DirectionDownload, Bytes: 1000, TrafficClass: TrafficInternet},
		{Timestamp: base, Direction: DirectionUpload, Bytes: 200, TrafficClass: TrafficInternet},
	} {
		if err := agg.Add(context.Background(), event, device, TrafficClassification{Class: TrafficInternet}); err != nil {
			t.Fatal(err)
		}
	}

	items, err := agg.Flush(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one aggregate, got %d", len(items))
	}
	if items[0].DownloadBytes != 1000 || items[0].UploadBytes != 200 {
		t.Fatalf("unexpected direction totals: %+v", items[0])
	}
	if items[0].ISPPeriod != string(isp.PeriodFreeNight) {
		t.Fatalf("expected free period, got %q", items[0].ISPPeriod)
	}
}

func TestMemoryAggregatorKeepsUnknownClassificationHonest(t *testing.T) {
	agg := NewMemoryAggregator(isp.DefaultWindowConfig())
	event := TrafficEvent{
		Timestamp:     time.Date(2026, 8, 15, 8, 0, 0, 0, time.UTC),
		SourceIP:      netip.MustParseAddr("192.168.50.21"),
		DestinationIP: netip.MustParseAddr("203.0.113.10"),
		Direction:     DirectionDownload,
		Bytes:         4096,
		TrafficClass:  TrafficInternet,
	}
	if err := agg.Add(context.Background(), event, devices.Device{ID: "dev_unknown"}, TrafficClassification{}); err != nil {
		t.Fatal(err)
	}
	items, err := agg.Flush(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one aggregate, got %d", len(items))
	}
	if items[0].Category != "unknown" || items[0].Confidence != ConfidenceUnknown {
		t.Fatalf("classification should remain unknown, got %+v", items[0])
	}
}

func mustLoadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
