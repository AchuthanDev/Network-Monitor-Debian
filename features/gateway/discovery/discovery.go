package discovery

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Report struct {
	DefaultRoute      *Route          `json:"default_route,omitempty"`
	WANInterface      string          `json:"wan_interface"`
	Interfaces        []Interface     `json:"interfaces"`
	Routes            []Route         `json:"routes"`
	DockerBridges     []DockerBridge  `json:"docker_bridges"`
	DockerNetworks    []DockerNetwork `json:"docker_networks"`
	DHCPListeners     []Listener      `json:"dhcp_listeners"`
	DNSListeners      []Listener      `json:"dns_listeners"`
	Nftables          NftablesState   `json:"nftables"`
	IPForwarding      ForwardingState `json:"ip_forwarding"`
	ToolAvailability  map[string]bool `json:"tool_availability"`
	SSHConnectionRisk string          `json:"ssh_connection_risk,omitempty"`
	Warnings          []string        `json:"warnings"`
}

type Interface struct {
	Name          string   `json:"name"`
	HardwareAddr  string   `json:"hardware_addr"`
	Flags         []string `json:"flags"`
	IPv4Addresses []string `json:"ipv4_addresses"`
	IPv6Addresses []string `json:"ipv6_addresses"`
}

type Route struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Source      string `json:"source,omitempty"`
	Metric      int    `json:"metric,omitempty"`
}

type DockerBridge struct {
	Name  string   `json:"name"`
	CIDRs []string `json:"cidrs"`
	State string   `json:"state"`
}

type DockerNetwork struct {
	Name    string   `json:"name"`
	Driver  string   `json:"driver"`
	Subnets []string `json:"subnets"`
}

type Listener struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     uint16 `json:"port"`
}

type NftablesState struct {
	Available bool   `json:"available"`
	Ruleset   string `json:"ruleset,omitempty"`
	Error     string `json:"error,omitempty"`
}

type ForwardingState struct {
	IPv4Enabled bool   `json:"ipv4_enabled"`
	IPv6Enabled bool   `json:"ipv6_enabled"`
	Error       string `json:"error,omitempty"`
}

func Discover(ctx context.Context) Report {
	report := Report{ToolAvailability: map[string]bool{}}
	report.Interfaces = discoverInterfaces(&report)
	report.Routes = discoverRoutes(ctx, &report)
	for _, route := range report.Routes {
		if route.Destination == "default" && report.DefaultRoute == nil {
			item := route
			report.DefaultRoute = &item
			report.WANInterface = route.Interface
		}
	}
	report.DockerBridges = discoverDockerBridges(report.Interfaces)
	report.DockerNetworks = discoverDockerNetworks(ctx, &report)
	report.DHCPListeners = discoverListeners(67)
	report.DNSListeners = append(discoverListeners(53), discoverListeners(853)...)
	report.Nftables = discoverNftables(ctx, &report)
	report.IPForwarding = discoverForwarding()
	report.SSHConnectionRisk = os.Getenv("SSH_CONNECTION")
	return report
}

func discoverInterfaces(report *Report) []Interface {
	items, err := net.Interfaces()
	if err != nil {
		report.Warnings = append(report.Warnings, "failed to inspect interfaces: "+err.Error())
		return nil
	}
	result := make([]Interface, 0, len(items))
	for _, item := range items {
		addresses, err := item.Addrs()
		if err != nil {
			report.Warnings = append(report.Warnings, "failed to inspect addresses for "+item.Name+": "+err.Error())
			continue
		}
		iface := Interface{
			Name:         item.Name,
			HardwareAddr: item.HardwareAddr.String(),
			Flags:        strings.Split(item.Flags.String(), "|"),
		}
		for _, address := range addresses {
			prefix, err := netip.ParsePrefix(address.String())
			if err != nil {
				continue
			}
			if prefix.Addr().Is4() {
				iface.IPv4Addresses = append(iface.IPv4Addresses, prefix.String())
			} else {
				iface.IPv6Addresses = append(iface.IPv6Addresses, prefix.String())
			}
		}
		result = append(result, iface)
	}
	return result
}

func discoverRoutes(ctx context.Context, report *Report) []Route {
	report.ToolAvailability["ip"] = commandAvailable("ip")
	if !report.ToolAvailability["ip"] {
		report.Warnings = append(report.Warnings, "ip command unavailable; active routes could not be inspected")
		return nil
	}
	output, err := exec.CommandContext(ctx, "ip", "-j", "route", "show", "table", "main").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "ip route failed: "+err.Error())
		return nil
	}

	var raw []struct {
		Dst     string `json:"dst"`
		Gateway string `json:"gateway"`
		Dev     string `json:"dev"`
		PrefSrc string `json:"prefsrc"`
		Metric  int    `json:"metric"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		report.Warnings = append(report.Warnings, "failed to parse ip route JSON: "+err.Error())
		return nil
	}
	routes := make([]Route, 0, len(raw))
	for _, item := range raw {
		dst := item.Dst
		if dst == "" {
			dst = "default"
		}
		routes = append(routes, Route{
			Destination: dst,
			Gateway:     item.Gateway,
			Interface:   item.Dev,
			Source:      item.PrefSrc,
			Metric:      item.Metric,
		})
	}
	return routes
}

func discoverDockerBridges(interfaces []Interface) []DockerBridge {
	var bridges []DockerBridge
	for _, iface := range interfaces {
		if iface.Name != "docker0" && !strings.HasPrefix(iface.Name, "br-") {
			continue
		}
		bridges = append(bridges, DockerBridge{
			Name:  iface.Name,
			CIDRs: iface.IPv4Addresses,
			State: interfaceState(iface.Flags),
		})
	}
	return bridges
}

func discoverDockerNetworks(ctx context.Context, report *Report) []DockerNetwork {
	report.ToolAvailability["docker"] = commandAvailable("docker")
	if !report.ToolAvailability["docker"] {
		report.Warnings = append(report.Warnings, "docker command unavailable; Docker network ranges could not be inspected")
		return nil
	}
	output, err := exec.CommandContext(ctx, "docker", "network", "ls", "--format", "{{.Name}}").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "docker network ls failed: "+err.Error())
		return nil
	}
	names := strings.Fields(string(output))
	networks := make([]DockerNetwork, 0, len(names))
	for _, name := range names {
		inspect, err := exec.CommandContext(ctx, "docker", "network", "inspect", name).Output()
		if err != nil {
			report.Warnings = append(report.Warnings, "docker network inspect "+name+" failed: "+err.Error())
			continue
		}
		var parsed []struct {
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
			IPAM   struct {
				Config []struct {
					Subnet string `json:"Subnet"`
				} `json:"Config"`
			} `json:"IPAM"`
		}
		if err := json.Unmarshal(inspect, &parsed); err != nil || len(parsed) == 0 {
			continue
		}
		network := DockerNetwork{Name: parsed[0].Name, Driver: parsed[0].Driver}
		for _, subnet := range parsed[0].IPAM.Config {
			if subnet.Subnet != "" {
				network.Subnets = append(network.Subnets, subnet.Subnet)
			}
		}
		networks = append(networks, network)
	}
	return networks
}

func discoverNftables(ctx context.Context, report *Report) NftablesState {
	report.ToolAvailability["nft"] = commandAvailable("nft")
	if !report.ToolAvailability["nft"] {
		return NftablesState{Available: false, Error: "nft command unavailable"}
	}
	output, err := exec.CommandContext(ctx, "nft", "list", "ruleset").CombinedOutput()
	if err != nil {
		return NftablesState{Available: true, Error: string(bytes.TrimSpace(output)) + " " + err.Error()}
	}
	return NftablesState{Available: true, Ruleset: string(output)}
}

func discoverForwarding() ForwardingState {
	ipv4, err4 := readBoolFile("/proc/sys/net/ipv4/ip_forward")
	ipv6, err6 := readBoolFile("/proc/sys/net/ipv6/conf/all/forwarding")
	state := ForwardingState{IPv4Enabled: ipv4, IPv6Enabled: ipv6}
	if err4 != nil {
		state.Error = err4.Error()
	}
	if err6 != nil {
		if state.Error != "" {
			state.Error += "; "
		}
		state.Error += err6.Error()
	}
	return state
}

func discoverListeners(port uint16) []Listener {
	var listeners []Listener
	for _, spec := range []struct {
		path     string
		protocol string
		ipv6     bool
	}{
		{path: "/proc/net/tcp", protocol: "tcp"},
		{path: "/proc/net/udp", protocol: "udp"},
		{path: "/proc/net/tcp6", protocol: "tcp6", ipv6: true},
		{path: "/proc/net/udp6", protocol: "udp6", ipv6: true},
	} {
		listeners = append(listeners, parseListeners(spec.path, spec.protocol, spec.ipv6, port)...)
	}
	return listeners
}

func parseListeners(path string, protocol string, ipv6 bool, port uint16) []Listener {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	var listeners []Listener
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[0] == "sl" {
			continue
		}
		address, parsedPort, ok := parseProcAddress(fields[1], ipv6)
		if !ok || parsedPort != port {
			continue
		}
		listeners = append(listeners, Listener{Protocol: protocol, Address: address, Port: parsedPort})
	}
	return listeners
}

func parseProcAddress(value string, ipv6 bool) (string, uint16, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	rawPort, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", 0, false
	}
	if ipv6 {
		return parts[0], uint16(rawPort), true
	}
	rawIP, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return "", 0, false
	}
	return net.IPv4(byte(rawIP), byte(rawIP>>8), byte(rawIP>>16), byte(rawIP>>24)).String(), uint16(rawPort), true
}

func commandAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func readBoolFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(raw)) == "1", nil
}

func interfaceState(flags []string) string {
	for _, flag := range flags {
		if flag == "up" {
			return "up"
		}
	}
	return "down"
}
