package readiness

import (
	"testing"

	gatewayconfig "github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/config"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/discovery"
)

func TestReadinessRequiresGigabitFullDuplexDedicatedLAN(t *testing.T) {
	cfg := gatewayconfig.Default()
	cfg.Gateway.WANInterface = "wan0"
	cfg.Gateway.LANInterface = "lan0"

	report := discovery.Report{
		WANInterface: "wan0",
		Interfaces: []discovery.Interface{
			{Name: "wan0", IPv4Addresses: []string{"192.168.1.10/24"}},
			{Name: "lan0", OperState: "up", Carrier: "1", SpeedMbps: 100, Duplex: "half"},
		},
		Nftables:     discovery.NftablesState{Available: true},
		IPForwarding: discovery.ForwardingState{IPv4Enabled: true},
	}

	ready := Evaluate(cfg, report)
	assertCheck(t, ready, "lan_link_speed", StatusFail)
	assertCheck(t, ready, "lan_full_duplex", StatusFail)
	if ready.Ready {
		t.Fatalf("readiness must fail for 100 Mbps half-duplex LAN")
	}
}

func TestReadinessPreservesSSHManagementOnWAN(t *testing.T) {
	cfg := gatewayconfig.Default()
	cfg.Gateway.WANInterface = "wan0"
	cfg.Gateway.LANInterface = "lan0"

	report := discovery.Report{
		WANInterface:      "wan0",
		SSHConnectionRisk: "192.168.1.2 55592 192.168.1.10 22",
		ToolAvailability:  map[string]bool{"docker": true},
		Nftables:          discovery.NftablesState{Available: true},
		IPForwarding:      discovery.ForwardingState{IPv4Enabled: true},
		Interfaces:        []discovery.Interface{{Name: "wan0", IPv4Addresses: []string{"192.168.1.10/24"}}, {Name: "lan0", OperState: "up", Carrier: "1", SpeedMbps: 1000, Duplex: "full", CandidateLAN: true}},
		DockerNetworks:    []discovery.DockerNetwork{{Name: "bridge", Subnets: []string{"172.17.0.0/16"}}},
		DHCPListeners:     nil,
		DNSListeners:      nil,
		DockerBridges:     nil,
		Routes:            []discovery.Route{{Destination: "192.168.1.0/24", Interface: "wan0"}},
		Warnings:          nil,
		DefaultRoute:      &discovery.Route{Destination: "default", Interface: "wan0", Gateway: "192.168.1.1"},
	}

	ready := Evaluate(cfg, report)
	assertCheck(t, ready, "ssh_management_preserved", StatusPass)
	assertCheck(t, ready, "rollback_plan_available", StatusPass)
	assertCheck(t, ready, "automatic_rollback_ready", StatusPass)
}

func assertCheck(t *testing.T, report Report, name string, status Status) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Status != status {
				t.Fatalf("%s status = %s, want %s (%s)", name, check.Status, status, check.Reason)
			}
			return
		}
	}
	t.Fatalf("missing readiness check %s", name)
}
