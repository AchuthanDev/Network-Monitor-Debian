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
	DefaultRoute      *Route            `json:"default_route,omitempty"`
	WANInterface      string            `json:"wan_interface"`
	Interfaces        []Interface       `json:"interfaces"`
	Routes            []Route           `json:"routes"`
	DockerBridges     []DockerBridge    `json:"docker_bridges"`
	DockerNetworks    []DockerNetwork   `json:"docker_networks"`
	DHCPListeners     []Listener        `json:"dhcp_listeners"`
	DNSListeners      []Listener        `json:"dns_listeners"`
	Nftables          NftablesState     `json:"nftables"`
	IPForwarding      ForwardingState   `json:"ip_forwarding"`
	ToolAvailability  map[string]bool   `json:"tool_availability"`
	ToolPaths         map[string]string `json:"tool_paths"`
	SSHConnectionRisk string            `json:"ssh_connection_risk,omitempty"`
	Warnings          []string          `json:"warnings"`
}

type Interface struct {
	Name            string   `json:"name"`
	HardwareAddr    string   `json:"hardware_addr"`
	Flags           []string `json:"flags"`
	Kind            string   `json:"kind,omitempty"`
	IPv4Addresses   []string `json:"ipv4_addresses"`
	IPv6Addresses   []string `json:"ipv6_addresses"`
	OperState       string   `json:"oper_state,omitempty"`
	Carrier         string   `json:"carrier,omitempty"`
	SpeedMbps       int      `json:"speed_mbps,omitempty"`
	Duplex          string   `json:"duplex,omitempty"`
	Driver          string   `json:"driver,omitempty"`
	Master          string   `json:"master,omitempty"`
	ManagedBy       string   `json:"managed_by,omitempty"`
	ConnectionName  string   `json:"connection_name,omitempty"`
	HasDefaultRoute bool     `json:"has_default_route"`
	Routes          []Route  `json:"routes,omitempty"`
	WiFi            *WiFi    `json:"wifi,omitempty"`
	CandidateLAN    bool     `json:"candidate_lan"`
	CandidateReason string   `json:"candidate_reason,omitempty"`
}

type WiFi struct {
	Phy                     string   `json:"phy,omitempty"`
	CurrentMode             string   `json:"current_mode,omitempty"`
	CurrentSSID             string   `json:"current_ssid,omitempty"`
	CurrentChannel          string   `json:"current_channel,omitempty"`
	APModeSupported         bool     `json:"ap_mode_supported"`
	SupportedInterfaceModes []string `json:"supported_interface_modes,omitempty"`
	Bands                   []string `json:"bands,omitempty"`
	Channels                []string `json:"channels,omitempty"`
	Concurrency             []string `json:"concurrency,omitempty"`
	Notes                   []string `json:"notes,omitempty"`
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
	report := Report{ToolAvailability: map[string]bool{}, ToolPaths: map[string]string{}}
	report.Interfaces = discoverInterfaces(&report)
	report.Routes = discoverRoutes(ctx, &report)
	for _, route := range report.Routes {
		if route.Destination == "default" && report.DefaultRoute == nil {
			item := route
			report.DefaultRoute = &item
			report.WANInterface = route.Interface
		}
	}
	report.Interfaces = annotateInterfaces(ctx, report.Interfaces, report.Routes, report.WANInterface, &report)
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
			OperState:    readTrimmed("/sys/class/net/" + item.Name + "/operstate"),
			Carrier:      readTrimmed("/sys/class/net/" + item.Name + "/carrier"),
			SpeedMbps:    readInt("/sys/class/net/" + item.Name + "/speed"),
			Duplex:       readTrimmed("/sys/class/net/" + item.Name + "/duplex"),
			Driver:       driverName(item.Name),
			Master:       linkBasename("/sys/class/net/" + item.Name + "/master"),
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

type nmDevice struct {
	kind       string
	state      string
	connection string
}

func annotateInterfaces(ctx context.Context, interfaces []Interface, routes []Route, wan string, report *Report) []Interface {
	defaults := map[string]bool{}
	for _, route := range routes {
		if route.Destination == "default" && route.Interface != "" {
			defaults[route.Interface] = true
		}
	}
	nm := discoverNetworkManagerDevices(ctx, report)
	wifi := discoverWiFi(ctx, report)
	for index := range interfaces {
		iface := &interfaces[index]
		for _, route := range routes {
			if route.Interface == iface.Name {
				iface.Routes = append(iface.Routes, route)
			}
		}
		if defaults[iface.Name] {
			iface.HasDefaultRoute = true
		}
		if item, ok := nm[iface.Name]; ok {
			iface.Kind = item.kind
			iface.ManagedBy = "NetworkManager"
			if item.connection != "" && item.connection != "--" {
				iface.ConnectionName = item.connection
			}
		}
		if item, ok := wifi[iface.Name]; ok {
			copy := item
			iface.WiFi = &copy
			if iface.Kind == "" {
				iface.Kind = "wifi"
			}
		}
		iface.CandidateLAN, iface.CandidateReason = lanCandidate(*iface, wan)
	}
	return interfaces
}

func discoverNetworkManagerDevices(ctx context.Context, report *Report) map[string]nmDevice {
	nmcli := commandPath(report, "nmcli")
	report.ToolAvailability["nmcli"] = nmcli != ""
	if !report.ToolAvailability["nmcli"] {
		return nil
	}
	output, err := exec.CommandContext(ctx, nmcli, "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "nmcli device status failed: "+err.Error())
		return nil
	}
	result := map[string]nmDevice{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		if len(parts) < 4 || parts[0] == "" {
			continue
		}
		result[parts[0]] = nmDevice{kind: parts[1], state: parts[2], connection: strings.Join(parts[3:], ":")}
	}
	return result
}

func discoverWiFi(ctx context.Context, report *Report) map[string]WiFi {
	iw := commandPath(report, "iw")
	report.ToolAvailability["iw"] = iw != ""
	if iw == "" {
		return nil
	}

	devOutput, err := exec.CommandContext(ctx, iw, "dev").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "iw dev failed: "+err.Error())
		return nil
	}
	listOutput, err := exec.CommandContext(ctx, iw, "list").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "iw list failed: "+err.Error())
		return nil
	}

	phyCaps := parseWiFiCapabilities(string(listOutput))
	result := map[string]WiFi{}
	currentPhy := ""
	currentIface := ""
	scanner := bufio.NewScanner(strings.NewReader(string(devOutput)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "phy#"):
			currentPhy = "phy" + strings.TrimPrefix(line, "phy#")
		case strings.HasPrefix(line, "Interface "):
			currentIface = strings.TrimSpace(strings.TrimPrefix(line, "Interface "))
			item := phyCaps[currentPhy]
			item.Phy = currentPhy
			result[currentIface] = item
		case currentIface != "" && strings.HasPrefix(line, "type "):
			item := result[currentIface]
			item.CurrentMode = strings.TrimSpace(strings.TrimPrefix(line, "type "))
			result[currentIface] = item
		case currentIface != "" && strings.HasPrefix(line, "ssid "):
			item := result[currentIface]
			item.CurrentSSID = strings.TrimSpace(strings.TrimPrefix(line, "ssid "))
			result[currentIface] = item
		case currentIface != "" && strings.HasPrefix(line, "channel "):
			item := result[currentIface]
			item.CurrentChannel = strings.TrimSpace(strings.TrimPrefix(line, "channel "))
			result[currentIface] = item
		}
	}
	return result
}

func parseWiFiCapabilities(raw string) map[string]WiFi {
	result := map[string]WiFi{}
	currentPhy := ""
	section := ""
	currentBand := ""
	lastConcurrencyIndex := -1
	scanner := bufio.NewScanner(strings.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Wiphy "):
			currentPhy = strings.TrimSpace(strings.TrimPrefix(trimmed, "Wiphy "))
			if _, ok := result[currentPhy]; !ok {
				result[currentPhy] = WiFi{Phy: currentPhy}
			}
			section = ""
			currentBand = ""
		case currentPhy == "":
			continue
		case strings.HasPrefix(trimmed, "Supported interface modes:"):
			section = "modes"
			lastConcurrencyIndex = -1
		case strings.HasPrefix(trimmed, "Band "):
			section = "band"
			lastConcurrencyIndex = -1
			currentBand = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(trimmed, "Band ")), ":")
			item := result[currentPhy]
			item.Bands = appendUnique(item.Bands, currentBand)
			result[currentPhy] = item
		case strings.HasPrefix(trimmed, "Frequencies:"):
			section = "frequencies"
			lastConcurrencyIndex = -1
		case strings.HasPrefix(trimmed, "valid interface combinations:"):
			section = "concurrency"
			lastConcurrencyIndex = -1
		case strings.HasPrefix(trimmed, "Supported commands:") ||
			strings.HasPrefix(trimmed, "Supported TX frame types:") ||
			strings.HasPrefix(trimmed, "Supported RX frame types:") ||
			strings.HasPrefix(trimmed, "Supported extended features:") ||
			strings.HasPrefix(trimmed, "HT Capability overrides:") ||
			strings.HasPrefix(trimmed, "Device supports ") ||
			strings.HasPrefix(trimmed, "Driver supports ") ||
			strings.HasPrefix(trimmed, "P2P GO supports ") ||
			strings.HasPrefix(trimmed, "max # scan plans:"):
			section = ""
			lastConcurrencyIndex = -1
		case section == "modes" && strings.HasPrefix(trimmed, "*"):
			mode := strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
			item := result[currentPhy]
			item.SupportedInterfaceModes = appendUnique(item.SupportedInterfaceModes, mode)
			if mode == "AP" {
				item.APModeSupported = true
			}
			result[currentPhy] = item
		case section == "frequencies" && strings.HasPrefix(trimmed, "*"):
			item := result[currentPhy]
			channel := currentBand + " " + strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
			item.Channels = append(item.Channels, channel)
			if strings.Contains(channel, "no IR") {
				item.Notes = appendUnique(item.Notes, "Some channels are marked no IR and may not be usable for initiating AP operation")
			}
			if strings.Contains(channel, "radar detection") {
				item.Notes = appendUnique(item.Notes, "DFS channels require radar detection support and careful country/channel configuration")
			}
			result[currentPhy] = item
		case section == "concurrency" && strings.HasPrefix(trimmed, "*"):
			item := result[currentPhy]
			item.Concurrency = append(item.Concurrency, strings.TrimSpace(strings.TrimPrefix(trimmed, "*")))
			lastConcurrencyIndex = len(item.Concurrency) - 1
			item = annotateWiFiConcurrency(item, item.Concurrency[lastConcurrencyIndex])
			result[currentPhy] = item
		case section == "concurrency" && lastConcurrencyIndex >= 0 && trimmed != "":
			item := result[currentPhy]
			item.Concurrency[lastConcurrencyIndex] += " " + trimmed
			item = annotateWiFiConcurrency(item, item.Concurrency[lastConcurrencyIndex])
			result[currentPhy] = item
		}
	}
	return result
}

func annotateWiFiConcurrency(item WiFi, combination string) WiFi {
	if strings.Contains(combination, "#{ AP,") && strings.Contains(combination, "#channels <= 1") {
		item.Notes = appendUnique(item.Notes, "AP plus managed/P2P concurrency is limited to one channel")
	}
	return item
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func lanCandidate(iface Interface, wan string) (bool, string) {
	switch {
	case iface.Name == "":
		return false, "unnamed interface"
	case iface.Name == wan:
		return false, "primary WAN/default-route interface"
	case iface.Name == "lo":
		return false, "loopback interface"
	case strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-") || strings.HasPrefix(iface.Name, "veth"):
		return false, "Docker or virtual interface"
	case iface.Master != "":
		return false, "interface is already enslaved to " + iface.Master
	case isWiFiInterface(iface):
		if iface.WiFi != nil && iface.WiFi.APModeSupported {
			return true, "Wi-Fi AP capable; usable as monitored LAN after disconnecting current managed Wi-Fi client state"
		}
		return false, "Wi-Fi AP mode was not confirmed by iw"
	case iface.HasDefaultRoute:
		return false, "interface already has a default route"
	case len(iface.IPv4Addresses) > 0:
		return false, "interface already has IPv4 configuration"
	case iface.OperState != "up":
		return false, "link is not up"
	case iface.SpeedMbps > 0 && iface.SpeedMbps < 1000:
		return false, "link speed is below 1 Gbps"
	case iface.Duplex != "" && iface.Duplex != "full":
		return false, "link duplex is not full"
	default:
		return true, "dedicated Ethernet candidate for monitored LAN"
	}
}

func isWiFiInterface(iface Interface) bool {
	return iface.Kind == "wifi" ||
		strings.HasPrefix(iface.Name, "wlp") ||
		strings.HasPrefix(iface.Name, "wl") ||
		strings.HasPrefix(iface.Name, "wifi")
}

func discoverRoutes(ctx context.Context, report *Report) []Route {
	ip := commandPath(report, "ip")
	report.ToolAvailability["ip"] = ip != ""
	if !report.ToolAvailability["ip"] {
		report.Warnings = append(report.Warnings, "ip command unavailable; active routes could not be inspected")
		return nil
	}
	output, err := exec.CommandContext(ctx, ip, "-j", "route", "show", "table", "main").Output()
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
	docker := commandPath(report, "docker")
	report.ToolAvailability["docker"] = docker != ""
	if !report.ToolAvailability["docker"] {
		report.Warnings = append(report.Warnings, "docker command unavailable; Docker network ranges could not be inspected")
		return nil
	}
	output, err := exec.CommandContext(ctx, docker, "network", "ls", "--format", "{{.Name}}").Output()
	if err != nil {
		report.Warnings = append(report.Warnings, "docker network ls failed: "+err.Error())
		return nil
	}
	names := strings.Fields(string(output))
	networks := make([]DockerNetwork, 0, len(names))
	for _, name := range names {
		inspect, err := exec.CommandContext(ctx, docker, "network", "inspect", name).Output()
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
	nft := commandPath(report, "nft")
	report.ToolAvailability["nft"] = nft != ""
	if !report.ToolAvailability["nft"] {
		return NftablesState{Available: false, Error: "nft command unavailable"}
	}
	output, err := exec.CommandContext(ctx, nft, "list", "ruleset").CombinedOutput()
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

func commandPath(report *Report, name string) string {
	if path, err := exec.LookPath(name); err == nil {
		report.ToolPaths[name] = path
		return path
	}
	for _, dir := range []string{"/usr/sbin", "/sbin", "/usr/bin", "/bin"} {
		path := dir + "/" + name
		if _, err := os.Stat(path); err == nil {
			report.ToolPaths[name] = path
			return path
		}
	}
	return ""
}

func readBoolFile(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(raw)) == "1", nil
}

func readTrimmed(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readInt(path string) int {
	value := readTrimmed(path)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func driverName(iface string) string {
	target, err := os.Readlink("/sys/class/net/" + iface + "/device/driver")
	if err != nil {
		return ""
	}
	return basename(target)
}

func linkBasename(path string) string {
	target, err := os.Readlink(path)
	if err != nil {
		return ""
	}
	return basename(target)
}

func basename(target string) string {
	parts := strings.Split(strings.Trim(target, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func interfaceState(flags []string) string {
	for _, flag := range flags {
		if flag == "up" {
			return "up"
		}
	}
	return "down"
}
