package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/plan"
)

type clientCounters []plan.ClientCounter

func main() {
	if len(os.Args) < 3 || os.Args[1] != "gateway" {
		usage()
		os.Exit(2)
	}

	switch os.Args[2] {
	case "plan":
		cfg := parseConfigFlags(os.Args[3:])
		writeJSON(plan.BuildDryRun(cfg))
	case "nftables":
		cfg, opts := parseNftablesFlags(os.Args[3:])
		fmt.Print(plan.RenderNftablesWithOptions(cfg, opts))
	case "rollback":
		cfg := parseConfigFlags(os.Args[3:])
		dryRunRequired("rollback", os.Args[3:])
		writeJSON(map[string]any{
			"status":            "dry_run",
			"rollback_commands": plan.BuildDryRun(cfg).RollbackCommands,
		})
	case "apply":
		parseConfigFlags(os.Args[3:])
		dryRunRequired("apply", os.Args[3:])
		writeJSON(map[string]string{
			"status":  "dry_run",
			"message": "live gateway apply is intentionally disabled pending explicit approval",
		})
	default:
		usage()
		os.Exit(2)
	}
}

func parseConfigFlags(args []string) gatewayconfig.Config {
	cfg := gatewayconfig.LoadFromEnv()
	mode := string(cfg.Mode)
	dnsMode := string(cfg.Gateway.DNS.Mode)
	fs := flag.NewFlagSet("network-monitor gateway", flag.ExitOnError)
	fs.StringVar(&mode, "mode", string(cfg.Mode), "gateway mode: server_only or gateway")
	fs.StringVar(&cfg.Gateway.WANInterface, "wan", cfg.Gateway.WANInterface, "WAN-side interface")
	fs.StringVar(&cfg.Gateway.LANInterface, "lan", cfg.Gateway.LANInterface, "monitored LAN interface")
	fs.StringVar(&cfg.Gateway.LANCIDR, "lan-cidr", cfg.Gateway.LANCIDR, "monitored LAN CIDR")
	fs.StringVar(&cfg.Gateway.GatewayIP, "gateway-ip", cfg.Gateway.GatewayIP, "monitored LAN gateway address")
	fs.StringVar(&cfg.Gateway.DHCP.RangeStart, "dhcp-start", cfg.Gateway.DHCP.RangeStart, "DHCP range start")
	fs.StringVar(&cfg.Gateway.DHCP.RangeEnd, "dhcp-end", cfg.Gateway.DHCP.RangeEnd, "DHCP range end")
	fs.BoolVar(&cfg.Gateway.DHCP.Enabled, "dhcp", cfg.Gateway.DHCP.Enabled, "include DHCP dry-run config")
	fs.BoolVar(&cfg.Gateway.AllowSlowLAN, "allow-slow-lan", cfg.Gateway.AllowSlowLAN, "allow monitored LAN links below 1 Gbps")
	fs.StringVar(&dnsMode, "dns-mode", string(cfg.Gateway.DNS.Mode), "DNS mode: disabled, forward, or pihole")
	fs.StringVar(&cfg.Timezone, "timezone", cfg.Timezone, "IANA timezone")
	fs.StringVar(&cfg.ISP.FreeWindow.Start, "free-start", cfg.ISP.FreeWindow.Start, "free window start HH:MM")
	fs.StringVar(&cfg.ISP.FreeWindow.End, "free-end", cfg.ISP.FreeWindow.End, "free window end HH:MM")
	_ = fs.Bool("dry-run", true, "accepted for apply/rollback; live changes are disabled")
	_ = fs.Parse(args)
	cfg.Mode = gatewayconfig.Mode(mode)
	cfg.Gateway.DNS.Mode = gatewayconfig.DNSMode(dnsMode)
	return cfg
}

func parseNftablesFlags(args []string) (gatewayconfig.Config, plan.NftablesOptions) {
	cfg := gatewayconfig.LoadFromEnv()
	mode := string(gatewayconfig.ModeGateway)
	var clients clientCounters
	fs := flag.NewFlagSet("network-monitor gateway nftables", flag.ExitOnError)
	fs.StringVar(&mode, "mode", string(gatewayconfig.ModeGateway), "gateway mode")
	fs.StringVar(&cfg.Gateway.WANInterface, "wan", cfg.Gateway.WANInterface, "WAN-side interface")
	fs.StringVar(&cfg.Gateway.LANInterface, "lan", cfg.Gateway.LANInterface, "monitored LAN interface")
	fs.StringVar(&cfg.Gateway.LANCIDR, "lan-cidr", cfg.Gateway.LANCIDR, "monitored LAN CIDR")
	fs.Var(&clients, "client-counter", "optional test/known-client counter in name=ipv4 form")
	_ = fs.Parse(args)
	cfg.Mode = gatewayconfig.Mode(mode)
	return cfg, plan.NftablesOptions{ClientCounters: clients}
}

func dryRunRequired(command string, args []string) {
	for _, arg := range args {
		if arg == "--dry-run" || arg == "-dry-run" {
			return
		}
	}
	fmt.Fprintf(os.Stderr, "gateway %s requires --dry-run on this branch\n", command)
	os.Exit(2)
}

func (c *clientCounters) String() string {
	return fmt.Sprint([]plan.ClientCounter(*c))
}

func (c *clientCounters) Set(value string) error {
	name, ip, ok := strings.Cut(value, "=")
	if !ok || name == "" || ip == "" {
		return fmt.Errorf("client counter must be name=ipv4")
	}
	*c = append(*c, plan.ClientCounter{Name: name, IPv4: ip})
	return nil
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: network-monitor gateway <plan|nftables|apply|rollback> [flags]")
}
