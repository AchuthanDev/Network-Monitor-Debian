package nft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"os/exec"
	"strings"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/accounting"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/network-usage/classifier"
)

const tableName = "network_monitor"

type CounterSnapshot struct {
	InternetDownload uint64
	InternetUpload   uint64
	LANDownload      uint64
	LANUpload        uint64
}

func (s CounterSnapshot) Deltas(previous CounterSnapshot) CounterSnapshot {
	return CounterSnapshot{
		InternetDownload: positiveDelta(s.InternetDownload, previous.InternetDownload),
		InternetUpload:   positiveDelta(s.InternetUpload, previous.InternetUpload),
		LANDownload:      positiveDelta(s.LANDownload, previous.LANDownload),
		LANUpload:        positiveDelta(s.LANUpload, previous.LANUpload),
	}
}

func (s CounterSnapshot) ToTrafficDeltas(now time.Time) []accounting.TrafficDelta {
	items := make([]accounting.TrafficDelta, 0, 2)
	if s.InternetDownload > 0 || s.InternetUpload > 0 {
		items = append(items, aggregateDelta(now, classifier.TrafficInternet, s.InternetDownload, s.InternetUpload))
	}
	if s.LANDownload > 0 || s.LANUpload > 0 {
		items = append(items, aggregateDelta(now, classifier.TrafficLAN, s.LANDownload, s.LANUpload))
	}
	return items
}

func Setup(ctx context.Context, defaultInterface string) error {
	if defaultInterface == "" {
		return fmt.Errorf("default interface is empty")
	}

	_ = run(ctx, "delete", "table", "inet", tableName)
	config := fmt.Sprintf(`
table inet %s {
  set private4 {
    type ipv4_addr
    flags interval
    elements = { 10.0.0.0/8, 127.0.0.0/8, 169.254.0.0/16, 172.16.0.0/12, 192.168.0.0/16, 224.0.0.0/4 }
  }

  counter internet_download {}
  counter internet_upload {}
  counter lan_download {}
  counter lan_upload {}

  chain input_account {
    type filter hook input priority -300; policy accept;
    iifname "%s" ip saddr @private4 counter name lan_download
    iifname "%s" ip saddr != @private4 counter name internet_download
  }

  chain output_account {
    type filter hook output priority -300; policy accept;
    oifname "%s" ip daddr @private4 counter name lan_upload
    oifname "%s" ip daddr != @private4 counter name internet_upload
  }

  chain forward_account {
    type filter hook forward priority -300; policy accept;
    iifname "%s" ip saddr @private4 counter name lan_download
    iifname "%s" ip saddr != @private4 counter name internet_download
    oifname "%s" ip daddr @private4 counter name lan_upload
    oifname "%s" ip daddr != @private4 counter name internet_upload
  }
}
`, tableName, defaultInterface, defaultInterface, defaultInterface, defaultInterface, defaultInterface, defaultInterface, defaultInterface, defaultInterface)

	cmd := exec.CommandContext(ctx, "nft", "-f", "-")
	cmd.Stdin = strings.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft setup: %w: %s", err, stderr.String())
	}
	return nil
}

func ReadCounters(ctx context.Context) (CounterSnapshot, error) {
	output, err := exec.CommandContext(ctx, "nft", "-j", "list", "counters", "table", "inet", tableName).Output()
	if err != nil {
		return CounterSnapshot{}, err
	}

	var parsed struct {
		Nftables []map[string]struct {
			Name  string `json:"name"`
			Bytes uint64 `json:"bytes"`
		} `json:"nftables"`
	}
	if err := json.Unmarshal(output, &parsed); err != nil {
		return CounterSnapshot{}, err
	}

	var snapshot CounterSnapshot
	for _, item := range parsed.Nftables {
		counter, ok := item["counter"]
		if !ok {
			continue
		}
		switch counter.Name {
		case "internet_download":
			snapshot.InternetDownload = counter.Bytes
		case "internet_upload":
			snapshot.InternetUpload = counter.Bytes
		case "lan_download":
			snapshot.LANDownload = counter.Bytes
		case "lan_upload":
			snapshot.LANUpload = counter.Bytes
		}
	}
	return snapshot, nil
}

func Cleanup(ctx context.Context) error {
	return run(ctx, "delete", "table", "inet", tableName)
}

func aggregateDelta(now time.Time, class classifier.TrafficClass, download uint64, upload uint64) accounting.TrafficDelta {
	return accounting.TrafficDelta{
		ObservedAt:            now.UTC(),
		Protocol:              "aggregate",
		LocalIP:               netip.IPv4Unspecified(),
		RemoteIP:              netip.IPv4Unspecified(),
		Class:                 class,
		DownloadBytes:         download,
		UploadBytes:           upload,
		AttributionSource:     "nftables",
		AttributionConfidence: "interface_counter",
		FlowKey:               "nftables:" + string(class),
	}
}

func positiveDelta(current uint64, previous uint64) uint64 {
	if current <= previous {
		return 0
	}
	return current - previous
}

func run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "nft", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}
