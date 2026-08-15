package config

import (
	"net/netip"
	"os"
	"strconv"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/isp"
)

type Mode string

const (
	ModeServerOnly Mode = "server_only"
	ModeGateway    Mode = "gateway"
)

type DNSMode string

const (
	DNSModeDisabled DNSMode = "disabled"
	DNSModeForward  DNSMode = "forward"
	DNSModePiHole   DNSMode = "pihole"
)

type LANMode string

const (
	LANModeEthernet LANMode = "ethernet"
	LANModeWiFiAP   LANMode = "wifi_ap"
)

type Config struct {
	Mode     Mode          `json:"mode"`
	Timezone string        `json:"timezone"`
	ISP      ISPConfig     `json:"isp"`
	Gateway  GatewayConfig `json:"gateway"`
}

type ISPConfig struct {
	FreeWindow TimeWindow `json:"free_window"`
}

type TimeWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type GatewayConfig struct {
	WANInterface string       `json:"wan_interface"`
	LANInterface string       `json:"lan_interface"`
	LANMode      LANMode      `json:"lan_mode"`
	LANCIDR      string       `json:"lan_cidr"`
	GatewayIP    string       `json:"gateway_ip"`
	AllowSlowLAN bool         `json:"allow_slow_lan"`
	DHCP         DHCPConfig   `json:"dhcp"`
	DNS          DNSConfig    `json:"dns"`
	WiFiAP       WiFiAPConfig `json:"wifi_ap"`
}

type WiFiAPConfig struct {
	TestMode      bool   `json:"test_mode"`
	SSID          string `json:"ssid"`
	CountryCode   string `json:"country_code"`
	Band          string `json:"band"`
	Channel       int    `json:"channel"`
	PassphraseEnv string `json:"passphrase_env"`
}

type DHCPConfig struct {
	Enabled    bool   `json:"enabled"`
	RangeStart string `json:"range_start"`
	RangeEnd   string `json:"range_end"`
}

type DNSConfig struct {
	Mode DNSMode `json:"mode"`
}

func Default() Config {
	window := isp.DefaultWindowConfig()
	return Config{
		Mode:     ModeServerOnly,
		Timezone: window.Timezone,
		ISP: ISPConfig{
			FreeWindow: TimeWindow{
				Start: window.FreeStart,
				End:   window.FreeEnd,
			},
		},
		Gateway: GatewayConfig{
			LANMode:   LANModeEthernet,
			LANCIDR:   "192.168.50.0/24",
			GatewayIP: "192.168.50.1",
			DHCP: DHCPConfig{
				Enabled:    false,
				RangeStart: "192.168.50.100",
				RangeEnd:   "192.168.50.240",
			},
			DNS: DNSConfig{Mode: DNSModeDisabled},
			WiFiAP: WiFiAPConfig{
				SSID:          "NetworkMonitor-Test",
				CountryCode:   "LK",
				Band:          "2.4ghz",
				Channel:       6,
				PassphraseEnv: "NETWORK_MONITOR_AP_PASSPHRASE",
			},
		},
	}
}

func LoadFromEnv() Config {
	cfg := Default()
	if value := os.Getenv("NETWORK_MONITOR_MODE"); value != "" {
		cfg.Mode = Mode(value)
	}
	if value := os.Getenv("NETWORK_MONITOR_TIMEZONE"); value != "" {
		cfg.Timezone = value
	}
	if value := os.Getenv("NETWORK_MONITOR_ISP_FREE_START"); value != "" {
		cfg.ISP.FreeWindow.Start = value
	}
	if value := os.Getenv("NETWORK_MONITOR_ISP_FREE_END"); value != "" {
		cfg.ISP.FreeWindow.End = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WAN_INTERFACE"); value != "" {
		cfg.Gateway.WANInterface = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_LAN_INTERFACE"); value != "" {
		cfg.Gateway.LANInterface = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_LAN_MODE"); value != "" {
		cfg.Gateway.LANMode = LANMode(value)
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_LAN_CIDR"); value != "" {
		cfg.Gateway.LANCIDR = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_IP"); value != "" {
		cfg.Gateway.GatewayIP = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_ALLOW_SLOW_LAN"); value == "true" {
		cfg.Gateway.AllowSlowLAN = true
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_DHCP_ENABLED"); value == "true" {
		cfg.Gateway.DHCP.Enabled = true
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_DHCP_RANGE_START"); value != "" {
		cfg.Gateway.DHCP.RangeStart = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_DHCP_RANGE_END"); value != "" {
		cfg.Gateway.DHCP.RangeEnd = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_DNS_MODE"); value != "" {
		cfg.Gateway.DNS.Mode = DNSMode(value)
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_TEST"); value == "true" {
		cfg.Gateway.LANMode = LANModeWiFiAP
		cfg.Gateway.WiFiAP.TestMode = true
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_SSID"); value != "" {
		cfg.Gateway.WiFiAP.SSID = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_COUNTRY"); value != "" {
		cfg.Gateway.WiFiAP.CountryCode = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_BAND"); value != "" {
		cfg.Gateway.WiFiAP.Band = value
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_CHANNEL"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			cfg.Gateway.WiFiAP.Channel = parsed
		}
	}
	if value := os.Getenv("NETWORK_MONITOR_GATEWAY_WIFI_AP_PASSPHRASE_ENV"); value != "" {
		cfg.Gateway.WiFiAP.PassphraseEnv = value
	}
	return cfg
}

func (c Config) ISPWindow() isp.WindowConfig {
	return isp.WindowConfig{
		Timezone:  c.Timezone,
		FreeStart: c.ISP.FreeWindow.Start,
		FreeEnd:   c.ISP.FreeWindow.End,
	}
}

func (c Config) LANPrefix() (netip.Prefix, error) {
	return netip.ParsePrefix(c.Gateway.LANCIDR)
}
