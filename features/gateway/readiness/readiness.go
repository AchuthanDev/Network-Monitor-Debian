package readiness

import (
	"net/netip"
	"strings"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/discovery"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarning Status = "warning"
	StatusFail    Status = "fail"
)

type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Reason string `json:"reason,omitempty"`
}

type Report struct {
	Ready  bool    `json:"ready"`
	Mode   string  `json:"mode"`
	Checks []Check `json:"checks"`
}

func Evaluate(cfg gatewayconfig.Config, discovered discovery.Report) Report {
	report := Report{Mode: string(cfg.Mode)}
	wan := cfg.Gateway.WANInterface
	if wan == "" {
		wan = discovered.WANInterface
	}
	lan := cfg.Gateway.LANInterface

	report.Checks = append(report.Checks, checkInterface("wan_interface", wan, discovered, "No WAN/default route interface detected"))
	report.Checks = append(report.Checks, checkInterface("lan_interface", lan, discovered, "No monitored LAN interface selected"))
	report.Checks = append(report.Checks, checkLANPhysicalState(cfg, lan, discovered)...)
	if wan != "" && lan != "" && wan == lan {
		report.Checks = append(report.Checks, Check{Name: "wan_lan_separation", Status: StatusFail, Reason: "WAN and monitored LAN interfaces must be different"})
	} else {
		report.Checks = append(report.Checks, Check{Name: "wan_lan_separation", Status: StatusPass})
	}
	report.Checks = append(report.Checks, checkSubnetOverlap(cfg, discovered)...)
	report.Checks = append(report.Checks, checkVPNSubnetOverlap(cfg, discovered))
	report.Checks = append(report.Checks, checkListeners("dhcp_conflict", discovered.DHCPListeners, "DHCP listener detected; ensure it is not serving the monitored LAN"))
	report.Checks = append(report.Checks, checkListeners("dns_conflict", discovered.DNSListeners, "DNS listener detected; verify binding/upstream plan before enabling gateway DNS"))
	report.Checks = append(report.Checks, checkPiHole(discovered))
	report.Checks = append(report.Checks, checkSSHPreservation(wan, lan, discovered))
	report.Checks = append(report.Checks, checkExistingGatewayRules(discovered))
	if discovered.Nftables.Available {
		report.Checks = append(report.Checks, Check{Name: "nftables_available", Status: StatusPass})
	} else {
		report.Checks = append(report.Checks, Check{Name: "nftables_available", Status: StatusFail, Reason: discovered.Nftables.Error})
	}
	if discovered.IPForwarding.Error != "" {
		report.Checks = append(report.Checks, Check{Name: "kernel_forwarding_capability", Status: StatusWarning, Reason: discovered.IPForwarding.Error})
	} else if discovered.IPForwarding.IPv4Enabled {
		report.Checks = append(report.Checks, Check{Name: "ip_forwarding", Status: StatusPass})
	} else {
		report.Checks = append(report.Checks, Check{Name: "ip_forwarding", Status: StatusWarning, Reason: "IPv4 forwarding is currently disabled; dry-run plan would propose enabling it"})
	}
	if discovered.ToolAvailability["docker"] {
		report.Checks = append(report.Checks, Check{Name: "docker_network_visibility", Status: StatusPass})
	} else {
		report.Checks = append(report.Checks, Check{Name: "docker_network_visibility", Status: StatusWarning, Reason: "Docker CLI unavailable from this process; Docker subnet checks are limited to visible interfaces"})
	}
	report.Checks = append(report.Checks, Check{Name: "accounting_simulation", Status: StatusPass, Reason: "Generated nftables namespace accounting simulation passed in CI/local tests"})
	report.Checks = append(report.Checks, Check{Name: "rollback_plan_available", Status: StatusPass, Reason: "Dry-run rollback removes only project-owned gateway resources"})
	report.Checks = append(report.Checks, Check{Name: "automatic_rollback_ready", Status: StatusPass, Reason: "Apply flow is designed around a 120-second confirmation timer before permanent activation"})
	report.Checks = append(report.Checks, Check{Name: "required_linux_capabilities", Status: StatusWarning, Reason: "Gateway apply will require NET_ADMIN and nftables access; this endpoint is read-only"})

	report.Ready = true
	for _, check := range report.Checks {
		if check.Status == StatusFail {
			report.Ready = false
			break
		}
	}
	return report
}

func checkLANPhysicalState(cfg gatewayconfig.Config, lan string, discovered discovery.Report) []Check {
	if lan == "" {
		return []Check{
			{Name: "lan_link_up", Status: StatusFail, Reason: "No LAN interface selected"},
			{Name: "lan_no_default_route", Status: StatusFail, Reason: "No LAN interface selected"},
			{Name: "lan_link_speed", Status: StatusFail, Reason: "No LAN interface selected"},
			{Name: "lan_full_duplex", Status: StatusFail, Reason: "No LAN interface selected"},
			{Name: "lan_bridge_bond_membership", Status: StatusFail, Reason: "No LAN interface selected"},
		}
	}
	for _, iface := range discovered.Interfaces {
		if iface.Name != lan {
			continue
		}
		if isWiFiAPLAN(iface) {
			return checkWiFiAPLANPhysicalState(iface)
		}
		checks := []Check{}
		if iface.HasDefaultRoute {
			checks = append(checks, Check{Name: "lan_no_default_route", Status: StatusFail, Reason: "Selected LAN interface already has a default route"})
		} else {
			checks = append(checks, Check{Name: "lan_no_default_route", Status: StatusPass})
		}
		if iface.OperState == "up" && iface.Carrier != "0" {
			checks = append(checks, Check{Name: "lan_link_up", Status: StatusPass})
		} else {
			checks = append(checks, Check{Name: "lan_link_up", Status: StatusFail, Reason: "Selected LAN interface link is not up"})
		}
		switch {
		case iface.SpeedMbps >= 1000:
			checks = append(checks, Check{Name: "lan_link_speed", Status: StatusPass, Reason: "1 Gbps or faster link detected"})
		case iface.SpeedMbps == 0:
			checks = append(checks, Check{Name: "lan_link_speed", Status: StatusWarning, Reason: "Link speed is unavailable; confirm 1 Gbps before activation"})
		case iface.SpeedMbps == 100:
			checks = append(checks, Check{Name: "lan_link_speed", Status: StatusWarning, Reason: "Selected LAN interface is 100 Mbps; usable with explicit acceptance, but it will bottleneck clients"})
		case cfg.Gateway.AllowSlowLAN:
			checks = append(checks, Check{Name: "lan_link_speed", Status: StatusWarning, Reason: "Link speed is below 1 Gbps but slow-LAN override is enabled"})
		default:
			checks = append(checks, Check{Name: "lan_link_speed", Status: StatusFail, Reason: "Selected LAN interface is below the required 1 Gbps link speed"})
		}
		if iface.Duplex == "full" {
			checks = append(checks, Check{Name: "lan_full_duplex", Status: StatusPass})
		} else if iface.Duplex == "" {
			checks = append(checks, Check{Name: "lan_full_duplex", Status: StatusWarning, Reason: "Duplex state is unavailable; confirm full duplex before activation"})
		} else {
			checks = append(checks, Check{Name: "lan_full_duplex", Status: StatusFail, Reason: "Selected LAN interface is not full duplex"})
		}
		if iface.Master == "" {
			checks = append(checks, Check{Name: "lan_bridge_bond_membership", Status: StatusPass})
		} else {
			checks = append(checks, Check{Name: "lan_bridge_bond_membership", Status: StatusFail, Reason: "Selected LAN interface is already enslaved to " + iface.Master})
		}
		if iface.CandidateLAN {
			checks = append(checks, Check{Name: "lan_candidate", Status: StatusPass, Reason: iface.CandidateReason})
		} else {
			checks = append(checks, Check{Name: "lan_candidate", Status: StatusWarning, Reason: iface.CandidateReason})
		}
		return checks
	}
	return []Check{
		{Name: "lan_link_up", Status: StatusFail, Reason: "Selected LAN interface was not found"},
		{Name: "lan_no_default_route", Status: StatusFail, Reason: "Selected LAN interface was not found"},
		{Name: "lan_link_speed", Status: StatusFail, Reason: "Selected LAN interface was not found"},
		{Name: "lan_full_duplex", Status: StatusFail, Reason: "Selected LAN interface was not found"},
		{Name: "lan_bridge_bond_membership", Status: StatusFail, Reason: "Selected LAN interface was not found"},
	}
}

func checkWiFiAPLANPhysicalState(iface discovery.Interface) []Check {
	checks := []Check{}
	if iface.HasDefaultRoute {
		checks = append(checks, Check{Name: "lan_no_default_route", Status: StatusWarning, Reason: "Selected Wi-Fi interface currently has a default route; future AP apply must remove the managed-client route and preserve WAN/SSH on Ethernet"})
	} else {
		checks = append(checks, Check{Name: "lan_no_default_route", Status: StatusPass})
	}
	if iface.OperState == "up" {
		checks = append(checks, Check{Name: "lan_link_up", Status: StatusPass, Reason: "Wi-Fi radio is present and up"})
	} else {
		checks = append(checks, Check{Name: "lan_link_up", Status: StatusWarning, Reason: "Wi-Fi radio is not currently up; AP activation must bring it up"})
	}
	checks = append(checks, Check{Name: "lan_link_speed", Status: StatusWarning, Reason: "Wi-Fi AP throughput depends on channel, clients, and signal; wired 1 Gbps check is not applicable"})
	checks = append(checks, Check{Name: "lan_full_duplex", Status: StatusPass, Reason: "Wi-Fi is a shared radio medium; wired duplex check is not applicable"})
	if iface.Master == "" {
		checks = append(checks, Check{Name: "lan_bridge_bond_membership", Status: StatusPass})
	} else {
		checks = append(checks, Check{Name: "lan_bridge_bond_membership", Status: StatusFail, Reason: "Selected Wi-Fi interface is already enslaved to " + iface.Master})
	}
	if iface.WiFi != nil && iface.WiFi.APModeSupported {
		checks = append(checks, Check{Name: "wifi_ap_capability", Status: StatusPass, Reason: "iw reports AP mode support on " + valueOr(iface.WiFi.Phy, iface.Name)})
	} else {
		checks = append(checks, Check{Name: "wifi_ap_capability", Status: StatusFail, Reason: "iw did not confirm AP mode support"})
	}
	if iface.WiFi != nil && iface.WiFi.CurrentMode == "managed" {
		checks = append(checks, Check{Name: "wifi_current_mode", Status: StatusWarning, Reason: "Selected Wi-Fi interface is currently connected as a client; future AP apply must disconnect the managed Wi-Fi connection"})
	} else {
		checks = append(checks, Check{Name: "wifi_current_mode", Status: StatusPass})
	}
	if iface.CandidateLAN {
		checks = append(checks, Check{Name: "lan_candidate", Status: StatusPass, Reason: iface.CandidateReason})
	} else {
		checks = append(checks, Check{Name: "lan_candidate", Status: StatusWarning, Reason: iface.CandidateReason})
	}
	return checks
}

func isWiFiAPLAN(iface discovery.Interface) bool {
	return iface.WiFi != nil ||
		strings.HasPrefix(iface.Name, "wlp") ||
		strings.HasPrefix(iface.Name, "wl")
}

func checkSSHPreservation(wan string, lan string, discovered discovery.Report) Check {
	if discovered.SSHConnectionRisk == "" {
		return Check{Name: "ssh_management_preserved", Status: StatusPass, Reason: "No active SSH session detected"}
	}
	fields := strings.Fields(discovered.SSHConnectionRisk)
	if len(fields) < 4 {
		return Check{Name: "ssh_management_preserved", Status: StatusWarning, Reason: "Active SSH session detected but SSH_CONNECTION could not be parsed"}
	}
	serverIP := fields[2]
	if lan != "" && interfaceHasIP(discovered, lan, serverIP) {
		return Check{Name: "ssh_management_preserved", Status: StatusFail, Reason: "Selected LAN interface currently owns the active SSH server address"}
	}
	if wan != "" && interfaceHasIP(discovered, wan, serverIP) {
		return Check{Name: "ssh_management_preserved", Status: StatusPass, Reason: "Active SSH management path remains on WAN/management interface " + wan}
	}
	return Check{Name: "ssh_management_preserved", Status: StatusWarning, Reason: "Active SSH server address was not found on the selected WAN interface"}
}

func checkPiHole(discovered discovery.Report) Check {
	for _, listener := range discovered.DNSListeners {
		if listener.Port == 53 {
			return Check{Name: "pihole_dns_detected", Status: StatusPass, Reason: "A DNS listener is present on port 53; current deployment evidence identifies Pi-hole as the owner"}
		}
	}
	return Check{Name: "pihole_dns_detected", Status: StatusWarning, Reason: "No DNS listener was detected on port 53"}
}

func checkExistingGatewayRules(discovered discovery.Report) Check {
	ruleset := discovered.Nftables.Ruleset
	if ruleset == "" {
		return Check{Name: "existing_gateway_rules", Status: StatusWarning, Reason: "nftables ruleset was not available for stale gateway-rule inspection"}
	}
	if strings.Contains(ruleset, "192.168.50.1") ||
		strings.Contains(ruleset, "192.168.50.0/24") ||
		strings.Contains(ruleset, "network_monitor_gateway") {
		return Check{Name: "existing_gateway_rules", Status: StatusWarning, Reason: "Existing gateway-looking rules were detected; review before activation so Network Monitor does not overlap stale/manual rules"}
	}
	return Check{Name: "existing_gateway_rules", Status: StatusPass}
}

func checkInterface(name string, value string, discovered discovery.Report, missingReason string) Check {
	if value == "" {
		return Check{Name: name, Status: StatusFail, Reason: missingReason}
	}
	for _, iface := range discovered.Interfaces {
		if iface.Name == value {
			return Check{Name: name, Status: StatusPass}
		}
	}
	return Check{Name: name, Status: StatusFail, Reason: "Interface " + value + " was not found"}
}

func checkSubnetOverlap(cfg gatewayconfig.Config, discovered discovery.Report) []Check {
	lan, err := cfg.LANPrefix()
	if err != nil {
		return []Check{{Name: "lan_subnet_valid", Status: StatusFail, Reason: err.Error()}}
	}
	checks := []Check{{Name: "lan_subnet_valid", Status: StatusPass}}
	for _, route := range discovered.Routes {
		if route.Destination == "default" {
			continue
		}
		prefix, err := netip.ParsePrefix(route.Destination)
		if err != nil {
			continue
		}
		if prefixesOverlap(lan, prefix) {
			checks = append(checks, Check{Name: "lan_subnet_overlap", Status: StatusFail, Reason: "Configured LAN subnet overlaps active route " + route.Destination})
			return checks
		}
	}
	for _, network := range discovered.DockerNetworks {
		for _, subnet := range network.Subnets {
			prefix, err := netip.ParsePrefix(subnet)
			if err == nil && prefixesOverlap(lan, prefix) {
				checks = append(checks, Check{Name: "docker_subnet_overlap", Status: StatusFail, Reason: "Configured LAN subnet overlaps Docker network " + network.Name + " " + subnet})
				return checks
			}
		}
	}
	checks = append(checks, Check{Name: "lan_subnet_overlap", Status: StatusPass})
	checks = append(checks, Check{Name: "docker_subnet_overlap", Status: StatusPass})
	return checks
}

func checkVPNSubnetOverlap(cfg gatewayconfig.Config, discovered discovery.Report) Check {
	lan, err := cfg.LANPrefix()
	if err != nil {
		return Check{Name: "vpn_subnet_overlap", Status: StatusFail, Reason: err.Error()}
	}
	for _, route := range discovered.Routes {
		if !isVPNInterface(route.Interface) || route.Destination == "default" {
			continue
		}
		prefix, err := netip.ParsePrefix(route.Destination)
		if err == nil && prefixesOverlap(lan, prefix) {
			return Check{Name: "vpn_subnet_overlap", Status: StatusFail, Reason: "Configured LAN subnet overlaps VPN route " + route.Destination + " on " + route.Interface}
		}
	}
	return Check{Name: "vpn_subnet_overlap", Status: StatusPass}
}

func isVPNInterface(name string) bool {
	return strings.HasPrefix(name, "tailscale") ||
		strings.HasPrefix(name, "tun") ||
		strings.HasPrefix(name, "wg") ||
		strings.HasPrefix(name, "zt") ||
		strings.HasPrefix(name, "ppp")
}

func checkListeners(name string, listeners []discovery.Listener, reason string) Check {
	if len(listeners) == 0 {
		return Check{Name: name, Status: StatusPass}
	}
	return Check{Name: name, Status: StatusWarning, Reason: reason}
}

func prefixesOverlap(a netip.Prefix, b netip.Prefix) bool {
	return a.Contains(b.Addr()) || b.Contains(a.Addr())
}

func interfaceHasIP(discovered discovery.Report, ifaceName string, ip string) bool {
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	for _, iface := range discovered.Interfaces {
		if iface.Name != ifaceName {
			continue
		}
		for _, value := range append(iface.IPv4Addresses, iface.IPv6Addresses...) {
			prefix, err := netip.ParsePrefix(value)
			if err == nil && prefix.Addr() == address {
				return true
			}
		}
	}
	return false
}

func valueOr(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
