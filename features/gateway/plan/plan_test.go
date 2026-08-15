package plan

import (
	"strings"
	"testing"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
)

func TestRenderNftablesUsesProjectOwnedTableAndPreNATAccounting(t *testing.T) {
	cfg := gatewayconfig.Default()
	cfg.Gateway.WANInterface = "wan0"
	cfg.Gateway.LANInterface = "lan0"
	cfg.Gateway.LANCIDR = "192.168.50.0/24"

	rules := RenderNftablesWithOptions(cfg, NftablesOptions{
		ClientCounters: []ClientCounter{{Name: "phone", IPv4: "192.168.50.21"}},
	})

	required := []string{
		"table inet network_monitor_gateway",
		"type filter hook forward priority -150",
		"chain nm_gateway_prenat_account",
		"chain nm_gateway_forward",
		"chain nm_gateway_nat",
		"type nat hook postrouting priority srcnat",
		"oifname \"wan0\" ip saddr @monitored_lan4 masquerade",
		"counter nm_gateway_phone_internet_download",
		"counter nm_gateway_phone_internet_upload",
		"ip saddr 192.168.50.21",
		"ip daddr 192.168.50.21",
	}
	for _, value := range required {
		if !strings.Contains(rules, value) {
			t.Fatalf("expected generated rules to contain %q:\n%s", value, rules)
		}
	}

	if strings.Contains(rules, "table ip nat") || strings.Contains(rules, "table ip filter") {
		t.Fatalf("generated rules must not target Docker/legacy global tables:\n%s", rules)
	}
}

func TestRenderDnsmasqIsDHCPOnly(t *testing.T) {
	cfg := gatewayconfig.Default()
	cfg.Gateway.LANInterface = "lan0"
	cfg.Gateway.DHCP.Enabled = true

	got := RenderDnsmasq(cfg)
	if !strings.Contains(got, "port=0") {
		t.Fatalf("dnsmasq config must disable DNS binding so Pi-hole owns port 53:\n%s", got)
	}
	if !strings.Contains(got, "interface=lan0") {
		t.Fatalf("dnsmasq config must bind to the monitored LAN interface:\n%s", got)
	}
}
