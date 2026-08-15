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
		"table inet network_monitor",
		"type filter hook forward priority -150",
		"type nat hook postrouting priority srcnat",
		"oifname \"wan0\" ip saddr @monitored_lan4 masquerade",
		"counter phone_internet_download",
		"counter phone_internet_upload",
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
