package devices

import (
	"net/netip"
	"testing"
	"time"
)

func TestSameMACGetsNewIPAndKeepsHistory(t *testing.T) {
	registry := NewRegistry(nil)
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)

	registry, first, created := registry.Upsert(Observation{
		MACAddress: "aa:bb:cc:dd:ee:ff",
		IP:         netip.MustParseAddr("192.168.50.21"),
		Hostname:   "phone",
		ObservedAt: now,
	})
	if !created {
		t.Fatal("expected first observation to create device")
	}

	registry, second, created := registry.Upsert(Observation{
		MACAddress: "AA-BB-CC-DD-EE-FF",
		IP:         netip.MustParseAddr("192.168.50.44"),
		Hostname:   "phone-renamed-by-dhcp",
		ObservedAt: now.Add(time.Hour),
	})
	if created {
		t.Fatal("same MAC should update existing device")
	}
	if first.ID != second.ID {
		t.Fatalf("history should stay linked by MAC: first=%+v second=%+v", first, second)
	}
	if second.CurrentIP.String() != "192.168.50.44" {
		t.Fatalf("expected current IP to update, got %+v", second)
	}
	if !second.FirstSeen.Equal(now) {
		t.Fatalf("first seen should be preserved, got %+v", second.FirstSeen)
	}
}

func TestUnknownMACCreatesNewDevice(t *testing.T) {
	registry := NewRegistry(nil)

	_, device, created := registry.Upsert(Observation{
		MACAddress: "11:22:33:44:55:66",
		IP:         netip.MustParseAddr("192.168.50.22"),
		ObservedAt: time.Date(2026, 8, 15, 11, 0, 0, 0, time.UTC),
	})
	if !created {
		t.Fatal("expected new MAC to create device")
	}
	if device.ID != "mac:11:22:33:44:55:66" {
		t.Fatalf("unexpected device ID: %+v", device)
	}
}

func TestDuplicateIPDoesNotMergeDifferentMACs(t *testing.T) {
	registry := NewRegistry(nil)
	ip := netip.MustParseAddr("192.168.50.21")

	registry, first, _ := registry.Upsert(Observation{
		MACAddress: "AA:BB:CC:DD:EE:FF",
		IP:         ip,
		ObservedAt: time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC),
	})
	_, second, _ := registry.Upsert(Observation{
		MACAddress: "11:22:33:44:55:66",
		IP:         ip,
		ObservedAt: time.Date(2026, 8, 15, 10, 1, 0, 0, time.UTC),
	})

	if first.ID == second.ID {
		t.Fatalf("duplicate IP must not merge different MACs: first=%+v second=%+v", first, second)
	}
}
