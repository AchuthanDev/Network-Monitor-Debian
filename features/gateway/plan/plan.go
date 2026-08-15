package plan

import (
	"fmt"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
)

type Plan struct {
	Mode             string   `json:"mode"`
	WouldChange      []string `json:"would_change"`
	NftablesRuleset  string   `json:"nftables_ruleset"`
	DnsmasqConfig    string   `json:"dnsmasq_config,omitempty"`
	RollbackCommands []string `json:"rollback_commands"`
	Warnings         []string `json:"warnings"`
}

func BuildDryRun(cfg gatewayconfig.Config) Plan {
	result := Plan{Mode: string(cfg.Mode)}
	if cfg.Mode != gatewayconfig.ModeGateway {
		result.Warnings = append(result.Warnings, "Gateway mode is not enabled; this is a planning preview only")
	}
	result.WouldChange = append(result.WouldChange,
		"enable IPv4 forwarding with sysctl net.ipv4.ip_forward=1",
		fmt.Sprintf("assign %s to LAN interface %s", cfg.Gateway.GatewayIP, cfg.Gateway.LANInterface),
		"create isolated nftables table inet network_monitor_gateway",
	)
	if cfg.Gateway.DHCP.Enabled {
		result.WouldChange = append(result.WouldChange, "write dnsmasq DHCP configuration for monitored LAN only")
	}
	result.NftablesRuleset = RenderNftables(cfg)
	result.DnsmasqConfig = RenderDnsmasq(cfg)
	result.RollbackCommands = []string{
		"nft delete table inet network_monitor_gateway",
		"systemctl stop network-monitor-dnsmasq.service",
		fmt.Sprintf("ip addr del %s dev %s", cfg.Gateway.GatewayIP, cfg.Gateway.LANInterface),
		"sysctl -w net.ipv4.ip_forward=<previous-value>",
	}
	return result
}

func RenderNftables(cfg gatewayconfig.Config) string {
	return fmt.Sprintf(`table inet network_monitor_gateway {
  set monitored_lan4 {
    type ipv4_addr
    flags interval
    elements = { %s }
  }

  counter client_internet_download {}
  counter client_internet_upload {}
  counter client_lan_download {}
  counter client_lan_upload {}

  chain forward_prenat_account {
    type filter hook forward priority -150; policy accept;
    iifname "%s" oifname "%s" ip saddr @monitored_lan4 ip daddr != @monitored_lan4 counter name client_internet_upload
    iifname "%s" oifname "%s" ip daddr @monitored_lan4 ip saddr != @monitored_lan4 counter name client_internet_download
    iifname "%s" ip saddr @monitored_lan4 ip daddr @monitored_lan4 counter name client_lan_upload
    oifname "%s" ip daddr @monitored_lan4 ip saddr @monitored_lan4 counter name client_lan_download
  }

  chain wan_nat {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "%s" ip saddr @monitored_lan4 masquerade
  }
}
`, cfg.Gateway.LANCIDR, cfg.Gateway.LANInterface, cfg.Gateway.WANInterface, cfg.Gateway.WANInterface, cfg.Gateway.LANInterface, cfg.Gateway.LANInterface, cfg.Gateway.LANInterface, cfg.Gateway.WANInterface)
}

func RenderDnsmasq(cfg gatewayconfig.Config) string {
	if !cfg.Gateway.DHCP.Enabled {
		return ""
	}
	return fmt.Sprintf(`interface=%s
bind-interfaces
dhcp-range=%s,%s,12h
dhcp-option=option:router,%s
dhcp-option=option:dns-server,%s
`, cfg.Gateway.LANInterface, cfg.Gateway.DHCP.RangeStart, cfg.Gateway.DHCP.RangeEnd, cfg.Gateway.GatewayIP, cfg.Gateway.GatewayIP)
}
