package discovery

import "testing"

func TestParseWiFiCapabilitiesDetectsAPAndConcurrency(t *testing.T) {
	raw := `Wiphy phy0
	Supported interface modes:
		 * managed
		 * AP
		 * P2P-client
	Band 1:
		Frequencies:
			* 2412.0 MHz [1] (22.0 dBm)
			* 2484.0 MHz [14] (disabled)
	valid interface combinations:
		 * #{ managed } <= 1, #{ AP, P2P-client, P2P-GO } <= 1, #{ P2P-device } <= 1,
		   total <= 3, #channels <= 1
`

	got := parseWiFiCapabilities(raw)["phy0"]
	if !got.APModeSupported {
		t.Fatalf("expected AP mode support: %+v", got)
	}
	if len(got.Channels) != 2 {
		t.Fatalf("expected channels to be parsed: %+v", got.Channels)
	}
	if len(got.Concurrency) != 1 {
		t.Fatalf("expected concurrency combination: %+v", got.Concurrency)
	}
	if len(got.Notes) == 0 || got.Notes[0] != "AP plus managed/P2P concurrency is limited to one channel" {
		t.Fatalf("expected AP concurrency note: %+v", got.Notes)
	}
	if got.Concurrency[0] != "#{ managed } <= 1, #{ AP, P2P-client, P2P-GO } <= 1, #{ P2P-device } <= 1, total <= 3, #channels <= 1" {
		t.Fatalf("expected continued concurrency line to be preserved: %+v", got.Concurrency)
	}
}
