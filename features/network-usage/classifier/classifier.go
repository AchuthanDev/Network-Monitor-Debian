package classifier

import (
	"fmt"
	"net/netip"
	"strings"
)

type TrafficClass string

const (
	TrafficInternet TrafficClass = "internet"
	TrafficLAN      TrafficClass = "lan"
	TrafficDocker   TrafficClass = "docker_internal"
	TrafficLoopback TrafficClass = "loopback"
	TrafficUnknown  TrafficClass = "unknown"
)

type Config struct {
	LANCIDRs    []netip.Prefix
	DockerCIDRs []netip.Prefix
}

type Result struct {
	Class            TrafficClass
	CountsAsInternet bool
	Reason           string
}

func DefaultConfig() Config {
	return Config{
		LANCIDRs: mustParsePrefixes(
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"127.0.0.0/8",
			"::1/128",
			"fc00::/7",
			"fe80::/10",
		),
		DockerCIDRs: mustParsePrefixes(
			"172.17.0.0/16",
			"172.18.0.0/16",
			"172.19.0.0/16",
			"172.20.0.0/16",
			"172.21.0.0/16",
		),
	}
}

func ParseCIDRList(raw string) ([]netip.Prefix, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("parse cidr %q: %w", part, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func ClassifyRemoteIP(remote netip.Addr, cfg Config) Result {
	if !remote.IsValid() {
		return Result{Class: TrafficUnknown, CountsAsInternet: false, Reason: "remote_ip_invalid"}
	}

	if remote.IsLoopback() {
		return Result{Class: TrafficLoopback, CountsAsInternet: false, Reason: "loopback_address"}
	}

	if contains(cfg.DockerCIDRs, remote) {
		return Result{Class: TrafficDocker, CountsAsInternet: false, Reason: "remote_in_docker_cidr"}
	}

	if contains(cfg.LANCIDRs, remote) || remote.IsPrivate() || remote.IsLinkLocalUnicast() || remote.IsMulticast() {
		return Result{Class: TrafficLAN, CountsAsInternet: false, Reason: "remote_in_lan_or_non_public_range"}
	}

	return Result{Class: TrafficInternet, CountsAsInternet: true, Reason: "remote_is_public_ip"}
}

func contains(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func mustParsePrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			panic(err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes
}
