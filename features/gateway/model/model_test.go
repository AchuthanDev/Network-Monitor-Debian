package model

import (
	"net/netip"
	"testing"
	"time"
)

func TestGatewayLANTransferDoesNotCountAsInternet(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()

	delta, ok, err := BuildGatewayDelta(FlowObservation{
		ObservedAt:       colomboTime(t, "2026-08-15T10:00:00+05:30"),
		FlowID:           "phone-to-plex",
		AccountingPoint:  AccountingPointForwardPreNAT,
		OriginalSourceIP: netip.MustParseAddr("192.168.50.21"),
		OriginalDestIP:   netip.MustParseAddr("192.168.50.10"),
		SourceMAC:        "AA:BB:CC:DD:EE:FF",
		OriginalBytes:    5_000_000,
		ReplyBytes:       900_000_000,
	}, cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected LAN delta")
	}
	if delta.TrafficScope != ScopeLAN || delta.CountsISP {
		t.Fatalf("LAN transfer must not count as Internet: %+v", delta)
	}
}

func TestGatewayPublicTrafficCountsPerDeviceBeforeNAT(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()

	delta, ok, err := BuildGatewayDelta(FlowObservation{
		ObservedAt:       colomboTime(t, "2026-08-15T12:00:00+05:30"),
		FlowID:           "phone-to-public",
		AccountingPoint:  AccountingPointForwardPreNAT,
		OriginalSourceIP: netip.MustParseAddr("192.168.50.21"),
		OriginalDestIP:   netip.MustParseAddr("8.8.8.8"),
		ReplyDestIP:      netip.MustParseAddr("192.168.50.21"),
		SourceMAC:        "AA-BB-CC-DD-EE-FF",
		OriginalBytes:    90_000_000,
		ReplyBytes:       1_400_000_000,
	}, cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected Internet delta")
	}
	if !delta.CountsISP || delta.TrafficScope != ScopeInternet {
		t.Fatalf("public remote should count as Internet: %+v", delta)
	}
	if delta.SourceID != "device-phone" {
		t.Fatalf("expected MAC-backed device identity, got %q", delta.SourceID)
	}
	if delta.UploadBytes != 90_000_000 || delta.DownloadBytes != 1_400_000_000 {
		t.Fatalf("unexpected direction accounting: %+v", delta)
	}
}

func TestTwoClientsAreCountedSeparately(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()
	observations := []FlowObservation{
		{
			ObservedAt:       colomboTime(t, "2026-08-15T12:00:00+05:30"),
			FlowID:           "phone",
			AccountingPoint:  AccountingPointForwardPreNAT,
			OriginalSourceIP: netip.MustParseAddr("192.168.50.21"),
			OriginalDestIP:   netip.MustParseAddr("8.8.8.8"),
			SourceMAC:        "AA:BB:CC:DD:EE:FF",
			OriginalBytes:    10,
			ReplyBytes:       90,
		},
		{
			ObservedAt:       colomboTime(t, "2026-08-15T12:01:00+05:30"),
			FlowID:           "tv",
			AccountingPoint:  AccountingPointForwardPreNAT,
			OriginalSourceIP: netip.MustParseAddr("192.168.50.22"),
			OriginalDestIP:   netip.MustParseAddr("1.1.1.1"),
			SourceMAC:        "11:22:33:44:55:66",
			OriginalBytes:    20,
			ReplyBytes:       180,
		},
	}

	var deltas []UsageDelta
	for _, observation := range observations {
		delta, ok, err := BuildGatewayDelta(observation, cfg, registry)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			deltas = append(deltas, delta)
		}
	}
	totals := SumInternetBySource(deltas)
	if totals["device-phone"] != 100 {
		t.Fatalf("phone total mismatch: %+v", totals)
	}
	if totals["device-tv"] != 200 {
		t.Fatalf("tv total mismatch: %+v", totals)
	}
}

func TestPostNATObservationIsNotAuthoritativeForDeviceAccounting(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()

	_, ok, err := BuildGatewayDelta(FlowObservation{
		ObservedAt:       colomboTime(t, "2026-08-15T12:00:00+05:30"),
		FlowID:           "post-nat-copy",
		AccountingPoint:  AccountingPointForwardPostNAT,
		OriginalSourceIP: netip.MustParseAddr("192.168.1.10"),
		OriginalDestIP:   netip.MustParseAddr("8.8.8.8"),
		SourceMAC:        "AA:BB:CC:DD:EE:FF",
		OriginalBytes:    10,
		ReplyBytes:       90,
	}, cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("post-NAT observation must not be accepted as authoritative per-device accounting")
	}
}

func TestForwardedFlowCountsOnlyAtAuthoritativeHook(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()
	points := []AccountingPoint{
		AccountingPointLANIngress,
		AccountingPointForwardPreNAT,
		AccountingPointWANEgress,
	}

	var accepted []UsageDelta
	for _, point := range points {
		delta, ok, err := BuildGatewayDelta(FlowObservation{
			ObservedAt:       colomboTime(t, "2026-08-15T12:00:00+05:30"),
			FlowID:           "same-forwarded-flow",
			AccountingPoint:  point,
			OriginalSourceIP: netip.MustParseAddr("192.168.50.21"),
			OriginalDestIP:   netip.MustParseAddr("8.8.8.8"),
			SourceMAC:        "AA:BB:CC:DD:EE:FF",
			OriginalBytes:    100,
			ReplyBytes:       900,
		}, cfg, registry)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			accepted = append(accepted, delta)
		}
	}

	if len(accepted) != 1 {
		t.Fatalf("expected exactly one authoritative accounting delta, got %d: %+v", len(accepted), accepted)
	}
	if accepted[0].DownloadBytes+accepted[0].UploadBytes != 1000 {
		t.Fatalf("unexpected authoritative total: %+v", accepted[0])
	}
}

func TestISPWindowBoundaryUsesConfigurableTimezone(t *testing.T) {
	cfg := gatewayTestConfig()
	cfg.ISPWindow = ISPWindowConfig{
		Timezone:  "Asia/Colombo",
		FreeStart: "00:00",
		FreeEnd:   "07:00",
	}

	free, err := cfg.ISPWindow.PeriodAt(colomboTime(t, "2026-08-15T06:59:00+05:30"))
	if err != nil {
		t.Fatal(err)
	}
	anytime, err := cfg.ISPWindow.PeriodAt(colomboTime(t, "2026-08-15T07:00:00+05:30"))
	if err != nil {
		t.Fatal(err)
	}
	if free != ISPPeriodFreeNight {
		t.Fatalf("06:59 should be free/night, got %s", free)
	}
	if anytime != ISPPeriodAnytime {
		t.Fatalf("07:00 should be anytime, got %s", anytime)
	}
}

func TestDeviceHistoryFollowsMACAcrossDHCPChange(t *testing.T) {
	registry := testRegistry()

	first := registry.Resolve(DeviceObservation{
		MAC: "AA:BB:CC:DD:EE:FF",
		IP:  netip.MustParseAddr("192.168.50.21"),
	})
	second := registry.Resolve(DeviceObservation{
		MAC: "AA:BB:CC:DD:EE:FF",
		IP:  netip.MustParseAddr("192.168.50.44"),
	})

	if first.ID != second.ID {
		t.Fatalf("device ID should follow MAC across DHCP IP change: first=%+v second=%+v", first, second)
	}
	if !first.Permanent || !second.Permanent {
		t.Fatalf("MAC-backed identities must be permanent: first=%+v second=%+v", first, second)
	}
}

func TestIPOnlyIdentityIsEphemeral(t *testing.T) {
	registry := testRegistry()

	device := registry.Resolve(DeviceObservation{IP: netip.MustParseAddr("192.168.50.88")})
	if device.Permanent {
		t.Fatalf("IP-only identity must not be treated as permanent: %+v", device)
	}
	if device.ID != "ephemeral-ip:192.168.50.88" {
		t.Fatalf("unexpected IP-only ID: %+v", device)
	}
}

func TestDockerTrafficDoesNotBecomeAttributedToLANDevice(t *testing.T) {
	cfg := gatewayTestConfig()
	registry := testRegistry()

	delta, ok, err := BuildGatewayDelta(FlowObservation{
		ObservedAt:       colomboTime(t, "2026-08-15T13:00:00+05:30"),
		FlowID:           "docker-peer",
		AccountingPoint:  AccountingPointForwardPreNAT,
		OriginalSourceIP: netip.MustParseAddr("172.20.0.4"),
		OriginalDestIP:   netip.MustParseAddr("172.20.0.5"),
		SourceMAC:        "AA:BB:CC:DD:EE:FF",
		OriginalBytes:    100,
		ReplyBytes:       200,
	}, cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected docker delta")
	}
	if delta.TrafficScope != ScopeDockerInternal || delta.CountsISP {
		t.Fatalf("docker traffic should remain internal: %+v", delta)
	}
}

func TestHostServerTrafficKeepsServerSource(t *testing.T) {
	cfg := gatewayTestConfig()

	delta, ok, err := BuildHostDelta(FlowObservation{
		ObservedAt:       colomboTime(t, "2026-08-15T13:00:00+05:30"),
		FlowID:           "server-public",
		AccountingPoint:  AccountingPointHostSocket,
		OriginalSourceIP: netip.MustParseAddr("192.168.1.10"),
		OriginalDestIP:   netip.MustParseAddr("8.8.8.8"),
		OriginalBytes:    300,
		ReplyBytes:       700,
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected host delta")
	}
	if delta.SourceType != "host" || delta.SourceID != "server" {
		t.Fatalf("server traffic must not be attributed to a LAN client: %+v", delta)
	}
	if !delta.CountsISP {
		t.Fatalf("server public traffic should still count as ISP usage: %+v", delta)
	}
}

func gatewayTestConfig() Config {
	cfg := DefaultConfig()
	cfg.Mode = ModeGateway
	cfg.MonitoredLAN = netip.MustParsePrefix("192.168.50.0/24")
	cfg.LANPrefixes = append(cfg.LANPrefixes, cfg.MonitoredLAN)
	cfg.ServerIPs = []netip.Addr{
		netip.MustParseAddr("192.168.1.10"),
		netip.MustParseAddr("192.168.50.1"),
	}
	return cfg
}

func testRegistry() DeviceRegistry {
	return NewDeviceRegistry([]DeviceIdentity{
		{ID: "device-phone", MAC: "AA:BB:CC:DD:EE:FF", FriendlyName: "Achuthan Phone", Type: "phone"},
		{ID: "device-tv", MAC: "11:22:33:44:55:66", FriendlyName: "Living Room TV", Type: "tv"},
	})
}

func colomboTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
