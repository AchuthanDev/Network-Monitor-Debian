package plan

import (
	"fmt"
	"strings"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
)

const NftablesTableName = "network_monitor"

type Plan struct {
	Mode             string   `json:"mode"`
	GatewayMode      string   `json:"gateway_mode"`
	WAN              string   `json:"wan"`
	LAN              string   `json:"lan"`
	LANIP            string   `json:"lan_ip"`
	DHCP             string   `json:"dhcp"`
	DNS              string   `json:"dns"`
	NAT              string   `json:"nat"`
	Accounting       string   `json:"accounting"`
	SSHManagement    string   `json:"ssh_management"`
	Safety           string   `json:"safety"`
	WouldChange      []string `json:"would_change"`
	NftablesRuleset  string   `json:"nftables_ruleset"`
	DnsmasqConfig    string   `json:"dnsmasq_config,omitempty"`
	RollbackCommands []string `json:"rollback_commands"`
	Warnings         []string `json:"warnings"`
}

func BuildDryRun(cfg gatewayconfig.Config) Plan {
	result := Plan{
		Mode:          string(cfg.Mode),
		GatewayMode:   string(cfg.Mode),
		WAN:           cfg.Gateway.WANInterface,
		LAN:           cfg.Gateway.LANInterface,
		LANIP:         gatewayAddressWithPrefix(cfg),
		DHCP:          fmt.Sprintf("%s-%s on %s", cfg.Gateway.DHCP.RangeStart, cfg.Gateway.DHCP.RangeEnd, cfg.Gateway.LANInterface),
		DNS:           string(cfg.Gateway.DNS.Mode),
		NAT:           fmt.Sprintf("%s -> %s", cfg.Gateway.LANCIDR, cfg.Gateway.WANInterface),
		Accounting:    "pre-NAT FORWARD hook in inet " + NftablesTableName,
		SSHManagement: "preserved through existing WAN/management interface; no SSH firewall changes are planned",
		Safety:        "live apply requires explicit approval plus a 120-second rollback confirmation timer",
	}
	if cfg.Mode != gatewayconfig.ModeGateway {
		result.Warnings = append(result.Warnings, "Gateway mode is not enabled; this is a planning preview only")
	}
	result.WouldChange = append(result.WouldChange,
		"enable IPv4 forwarding with sysctl net.ipv4.ip_forward=1",
		fmt.Sprintf("assign %s to LAN interface %s", cfg.Gateway.GatewayIP, cfg.Gateway.LANInterface),
		"create isolated nftables table inet "+NftablesTableName,
		fmt.Sprintf("masquerade monitored LAN %s to WAN interface %s", cfg.Gateway.LANCIDR, cfg.Gateway.WANInterface),
		"account client upload/download once at the pre-NAT forward hook",
		"preserve SSH management on the existing WAN/management interface",
		"start a 120-second rollback timer before any future live apply can be confirmed",
	)
	if cfg.Gateway.DHCP.Enabled {
		result.WouldChange = append(result.WouldChange, "write dnsmasq DHCP configuration for monitored LAN only")
	}
	result.NftablesRuleset = RenderNftables(cfg)
	result.DnsmasqConfig = RenderDnsmasq(cfg)
	result.RollbackCommands = []string{
		"nft delete table inet " + NftablesTableName,
		"systemctl stop network-monitor-dnsmasq.service",
		fmt.Sprintf("ip addr del %s dev %s", gatewayAddressWithPrefix(cfg), cfg.Gateway.LANInterface),
		"sysctl -w net.ipv4.ip_forward=<previous-value>",
	}
	return result
}

func RenderNftables(cfg gatewayconfig.Config) string {
	return RenderNftablesWithOptions(cfg, NftablesOptions{})
}

type NftablesOptions struct {
	ClientCounters []ClientCounter
}

type ClientCounter struct {
	Name string
	IPv4 string
}

func RenderNftablesWithOptions(cfg gatewayconfig.Config, opts NftablesOptions) string {
	var clientCounters strings.Builder
	for _, client := range opts.ClientCounters {
		if client.Name == "" || client.IPv4 == "" {
			continue
		}
		name := sanitizeCounterName(client.Name)
		clientCounters.WriteString(fmt.Sprintf("  counter %s_internet_download {}\n", name))
		clientCounters.WriteString(fmt.Sprintf("  counter %s_internet_upload {}\n", name))
	}

	var clientRules strings.Builder
	for _, client := range opts.ClientCounters {
		if client.Name == "" || client.IPv4 == "" {
			continue
		}
		name := sanitizeCounterName(client.Name)
		clientRules.WriteString(fmt.Sprintf("    iifname \"%s\" oifname \"%s\" ip saddr %s ip daddr != @monitored_lan4 counter name %s_internet_upload\n", cfg.Gateway.LANInterface, cfg.Gateway.WANInterface, client.IPv4, name))
		clientRules.WriteString(fmt.Sprintf("    iifname \"%s\" oifname \"%s\" ip daddr %s ip saddr != @monitored_lan4 counter name %s_internet_download\n", cfg.Gateway.WANInterface, cfg.Gateway.LANInterface, client.IPv4, name))
	}

	return fmt.Sprintf(`table inet %s {
  set monitored_lan4 {
    type ipv4_addr
    flags interval
    elements = { %s }
  }

  counter client_internet_download {}
  counter client_internet_upload {}
  counter client_lan_download {}
  counter client_lan_upload {}
%s

  chain forward_prenat_account {
    type filter hook forward priority -150; policy accept;
%s    meta nfproto ipv4 iifname "%s" oifname "%s" ip saddr @monitored_lan4 ip daddr != @monitored_lan4 counter name client_internet_upload
    meta nfproto ipv4 iifname "%s" oifname "%s" ip daddr @monitored_lan4 ip saddr != @monitored_lan4 counter name client_internet_download
    meta nfproto ipv4 iifname "%s" ip saddr @monitored_lan4 ip daddr @monitored_lan4 counter name client_lan_upload
    meta nfproto ipv4 oifname "%s" ip daddr @monitored_lan4 ip saddr @monitored_lan4 counter name client_lan_download
  }

  chain gateway_forward {
    type filter hook forward priority 0; policy accept;
    ct state established,related accept
    iifname "%s" oifname "%s" ip saddr @monitored_lan4 accept
  }

  chain wan_nat {
    type nat hook postrouting priority srcnat; policy accept;
    oifname "%s" ip saddr @monitored_lan4 masquerade
  }
}
`, NftablesTableName, cfg.Gateway.LANCIDR, clientCounters.String(), clientRules.String(), cfg.Gateway.LANInterface, cfg.Gateway.WANInterface, cfg.Gateway.WANInterface, cfg.Gateway.LANInterface, cfg.Gateway.LANInterface, cfg.Gateway.LANInterface, cfg.Gateway.LANInterface, cfg.Gateway.WANInterface, cfg.Gateway.WANInterface)
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

func sanitizeCounterName(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	if b.Len() == 0 {
		return "client"
	}
	return b.String()
}

func gatewayAddressWithPrefix(cfg gatewayconfig.Config) string {
	prefix, err := cfg.LANPrefix()
	if err != nil {
		return cfg.Gateway.GatewayIP
	}
	return fmt.Sprintf("%s/%d", cfg.Gateway.GatewayIP, prefix.Bits())
}
