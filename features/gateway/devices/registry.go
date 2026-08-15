package devices

import (
	"net"
	"net/netip"
	"strings"
	"time"
)

type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusUnknown Status = "unknown"
)

type Device struct {
	ID           string     `json:"id"`
	MACAddress   string     `json:"mac_address"`
	CurrentIP    netip.Addr `json:"current_ip"`
	Hostname     string     `json:"hostname"`
	FriendlyName string     `json:"friendly_name"`
	DeviceType   string     `json:"device_type"`
	Manufacturer string     `json:"manufacturer"`
	FirstSeen    time.Time  `json:"first_seen"`
	LastSeen     time.Time  `json:"last_seen"`
	Status       Status     `json:"status"`
}

type Observation struct {
	MACAddress   string
	IP           netip.Addr
	Hostname     string
	Manufacturer string
	ObservedAt   time.Time
}

type Registry struct {
	byMAC map[string]Device
}

func NewRegistry(existing []Device) Registry {
	registry := Registry{byMAC: map[string]Device{}}
	for _, device := range existing {
		normalized := NormalizeMAC(device.MACAddress)
		if normalized == "" {
			continue
		}
		device.MACAddress = normalized
		if device.ID == "" {
			device.ID = idForMAC(normalized)
		}
		registry.byMAC[normalized] = device
	}
	return registry
}

func (r Registry) Upsert(observation Observation) (Registry, Device, bool) {
	normalized := NormalizeMAC(observation.MACAddress)
	if normalized == "" {
		now := observationTime(observation.ObservedAt)
		device := Device{
			ID:           "unknown:" + observation.IP.String(),
			CurrentIP:    observation.IP,
			Hostname:     observation.Hostname,
			Manufacturer: observation.Manufacturer,
			FirstSeen:    now,
			LastSeen:     now,
			Status:       StatusUnknown,
		}
		return r, device, true
	}

	if r.byMAC == nil {
		r.byMAC = map[string]Device{}
	}

	now := observationTime(observation.ObservedAt)
	device, exists := r.byMAC[normalized]
	if !exists {
		device = Device{
			ID:         idForMAC(normalized),
			MACAddress: normalized,
			FirstSeen:  now,
		}
	}

	device.CurrentIP = observation.IP
	if observation.Hostname != "" {
		device.Hostname = observation.Hostname
	}
	if observation.Manufacturer != "" {
		device.Manufacturer = observation.Manufacturer
	}
	if device.FirstSeen.IsZero() {
		device.FirstSeen = now
	}
	device.LastSeen = now
	device.Status = StatusOnline
	r.byMAC[normalized] = device
	return r, device, !exists
}

func NormalizeMAC(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	mac, err := net.ParseMAC(value)
	if err != nil {
		return ""
	}
	return strings.ToUpper(mac.String())
}

func observationTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func idForMAC(mac string) string {
	return "mac:" + mac
}
