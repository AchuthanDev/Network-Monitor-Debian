package model

import (
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
)

type Mode string

const (
	ModeServerOnly Mode = "server_only"
	ModeGateway    Mode = "gateway"
)

type TrafficScope string

const (
	ScopeInternet       TrafficScope = "internet"
	ScopeLAN            TrafficScope = "lan"
	ScopeServerLocal    TrafficScope = "server_local"
	ScopeDockerInternal TrafficScope = "docker_internal"
	ScopeUnknown        TrafficScope = "unknown"
)

type Direction string

const (
	DirectionDownload Direction = "download"
	DirectionUpload   Direction = "upload"
)

type AccountingPoint string

const (
	AccountingPointForwardPreNAT AccountingPoint = "forward_prenat"
	AccountingPointForwardPostNAT AccountingPoint = "forward_postnat"
	AccountingPointLANIngress     AccountingPoint = "lan_ingress"
	AccountingPointWANEgress      AccountingPoint = "wan_egress"
	AccountingPointHostSocket     AccountingPoint = "host_socket"
	AccountingPointDockerBridge   AccountingPoint = "docker_bridge"
)

type ISPPeriod string

const (
	ISPPeriodFreeNight ISPPeriod = "free_night"
	ISPPeriodAnytime   ISPPeriod = "anytime"
)

type DeviceIdentity struct {
	ID           string
	MAC          string
	IP           netip.Addr
	Hostname     string
	FriendlyName string
	Type         string
	Permanent    bool
}

type DeviceObservation struct {
	MAC      string
	IP       netip.Addr
	Hostname string
}

type DeviceRegistry struct {
	byMAC map[string]DeviceIdentity
}

func NewDeviceRegistry(devices []DeviceIdentity) DeviceRegistry {
	registry := DeviceRegistry{byMAC: map[string]DeviceIdentity{}}
	for _, device := range devices {
		normalized := normalizeMAC(device.MAC)
		if normalized == "" {
			continue
		}
		device.MAC = normalized
		if device.ID == "" {
			device.ID = "mac:" + normalized
		}
		device.Permanent = true
		registry.byMAC[normalized] = device
	}
	return registry
}

func (r DeviceRegistry) Resolve(observation DeviceObservation) DeviceIdentity {
	normalizedMAC := normalizeMAC(observation.MAC)
	if normalizedMAC != "" {
		device, ok := r.byMAC[normalizedMAC]
		if !ok {
			device = DeviceIdentity{ID: "mac:" + normalizedMAC, MAC: normalizedMAC, Permanent: true}
		}
		device.IP = observation.IP
		if observation.Hostname != "" {
			device.Hostname = observation.Hostname
		}
		return device
	}

	id := "unknown"
	if observation.IP.IsValid() {
		id = "ephemeral-ip:" + observation.IP.String()
	}
	return DeviceIdentity{
		ID:        id,
		IP:        observation.IP,
		Hostname:  observation.Hostname,
		Permanent: false,
	}
}

type ISPWindowConfig struct {
	Timezone  string
	FreeStart string
	FreeEnd   string
}

func DefaultISPWindowConfig() ISPWindowConfig {
	return ISPWindowConfig{
		Timezone:  "Asia/Colombo",
		FreeStart: "00:00",
		FreeEnd:   "07:00",
	}
}

func (c ISPWindowConfig) PeriodAt(observedAt time.Time) (ISPPeriod, error) {
	location, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return "", err
	}
	start, err := parseClock(c.FreeStart)
	if err != nil {
		return "", err
	}
	end, err := parseClock(c.FreeEnd)
	if err != nil {
		return "", err
	}

	local := observedAt.In(location)
	minute := local.Hour()*60 + local.Minute()
	if inWindow(minute, start, end) {
		return ISPPeriodFreeNight, nil
	}
	return ISPPeriodAnytime, nil
}

type Config struct {
	Mode              Mode
	LANPrefixes       []netip.Prefix
	DockerPrefixes    []netip.Prefix
	ServerIPs         []netip.Addr
	MonitoredLAN      netip.Prefix
	AuthoritativeHook AccountingPoint
	ISPWindow         ISPWindowConfig
}

func DefaultConfig() Config {
	networkConfig := classifier.DefaultConfig()
	return Config{
		Mode:              ModeServerOnly,
		LANPrefixes:       networkConfig.LANCIDRs,
		DockerPrefixes:    networkConfig.DockerCIDRs,
		ServerIPs:         nil,
		AuthoritativeHook: AccountingPointForwardPreNAT,
		ISPWindow:         DefaultISPWindowConfig(),
	}
}

func (c Config) classifierConfig() classifier.Config {
	return classifier.Config{
		LANCIDRs:    c.LANPrefixes,
		DockerCIDRs: c.DockerPrefixes,
	}
}

type FlowObservation struct {
	ObservedAt       time.Time
	FlowID           string
	AccountingPoint  AccountingPoint
	OriginalSourceIP netip.Addr
	OriginalDestIP   netip.Addr
	ReplySourceIP    netip.Addr
	ReplyDestIP      netip.Addr
	SourceMAC        string
	SourceHostname   string
	OriginalBytes    uint64
	ReplyBytes       uint64
}

type UsageDelta struct {
	ObservedAt    time.Time
	FlowID        string
	SourceType    string
	SourceID      string
	SourceIP      netip.Addr
	SourceMAC     string
	TrafficScope  TrafficScope
	DownloadBytes uint64
	UploadBytes   uint64
	ISPPeriod     ISPPeriod
	CountsISP     bool
}

func BuildGatewayDelta(observation FlowObservation, cfg Config, registry DeviceRegistry) (UsageDelta, bool, error) {
	if observation.AccountingPoint != cfg.AuthoritativeHook {
		return UsageDelta{}, false, nil
	}
	if observation.OriginalBytes == 0 && observation.ReplyBytes == 0 {
		return UsageDelta{}, false, nil
	}

	scope := classifyScope(observation, cfg)
	period := ISPPeriodAnytime
	if scope == ScopeInternet {
		var err error
		period, err = cfg.ISPWindow.PeriodAt(observation.ObservedAt)
		if err != nil {
			return UsageDelta{}, false, err
		}
	}

	device := registry.Resolve(DeviceObservation{
		MAC:      observation.SourceMAC,
		IP:       observation.OriginalSourceIP,
		Hostname: observation.SourceHostname,
	})

	return UsageDelta{
		ObservedAt:    observation.ObservedAt.UTC(),
		FlowID:        observation.FlowID,
		SourceType:    "device",
		SourceID:      device.ID,
		SourceIP:      device.IP,
		SourceMAC:     device.MAC,
		TrafficScope:  scope,
		UploadBytes:   observation.OriginalBytes,
		DownloadBytes: observation.ReplyBytes,
		ISPPeriod:     period,
		CountsISP:     scope == ScopeInternet,
	}, true, nil
}

func BuildHostDelta(observation FlowObservation, cfg Config) (UsageDelta, bool, error) {
	if observation.AccountingPoint != AccountingPointHostSocket {
		return UsageDelta{}, false, nil
	}
	if observation.OriginalBytes == 0 && observation.ReplyBytes == 0 {
		return UsageDelta{}, false, nil
	}

	scope := classifyScope(observation, cfg)
	period := ISPPeriodAnytime
	if scope == ScopeInternet {
		var err error
		period, err = cfg.ISPWindow.PeriodAt(observation.ObservedAt)
		if err != nil {
			return UsageDelta{}, false, err
		}
	}
	return UsageDelta{
		ObservedAt:    observation.ObservedAt.UTC(),
		FlowID:        observation.FlowID,
		SourceType:    "host",
		SourceID:      "server",
		SourceIP:      observation.OriginalSourceIP,
		TrafficScope:  scope,
		UploadBytes:   observation.OriginalBytes,
		DownloadBytes: observation.ReplyBytes,
		ISPPeriod:     period,
		CountsISP:     scope == ScopeInternet,
	}, true, nil
}

func SumInternetBySource(deltas []UsageDelta) map[string]uint64 {
	totals := map[string]uint64{}
	for _, delta := range deltas {
		if !delta.CountsISP {
			continue
		}
		totals[delta.SourceID] += delta.DownloadBytes + delta.UploadBytes
	}
	return totals
}

func classifyScope(observation FlowObservation, cfg Config) TrafficScope {
	if containsAddr(cfg.ServerIPs, observation.OriginalSourceIP) || containsAddr(cfg.ServerIPs, observation.OriginalDestIP) {
		if observation.AccountingPoint == AccountingPointHostSocket {
			return fromTrafficClass(classifier.ClassifyRemoteIP(observation.OriginalDestIP, cfg.classifierConfig()).Class)
		}
		return ScopeServerLocal
	}

	if containsPrefix(cfg.DockerPrefixes, observation.OriginalSourceIP) || containsPrefix(cfg.DockerPrefixes, observation.OriginalDestIP) {
		return ScopeDockerInternal
	}

	classification := classifier.ClassifyRemoteIP(observation.OriginalDestIP, cfg.classifierConfig())
	return fromTrafficClass(classification.Class)
}

func fromTrafficClass(class classifier.TrafficClass) TrafficScope {
	switch class {
	case classifier.TrafficInternet:
		return ScopeInternet
	case classifier.TrafficLAN:
		return ScopeLAN
	case classifier.TrafficDocker:
		return ScopeDockerInternal
	case classifier.TrafficLoopback:
		return ScopeServerLocal
	default:
		return ScopeUnknown
	}
}

func containsPrefix(prefixes []netip.Prefix, addr netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func containsAddr(addrs []netip.Addr, addr netip.Addr) bool {
	for _, item := range addrs {
		if item == addr {
			return true
		}
	}
	return false
}

func normalizeMAC(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	mac, err := net.ParseMAC(value)
	if err != nil {
		return ""
	}
	return strings.ToUpper(mac.String())
}

func parseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("parse clock %q: %w", value, err)
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func inWindow(minute int, start int, end int) bool {
	if start == end {
		return false
	}
	if start < end {
		return minute >= start && minute < end
	}
	return minute >= start || minute < end
}
