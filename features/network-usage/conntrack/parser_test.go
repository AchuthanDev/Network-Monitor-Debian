package conntrack

import (
	"strings"
	"testing"
)

func TestParseTCPLineWithCounters(t *testing.T) {
	line := "ipv4 2 tcp 6 431999 ESTABLISHED src=192.168.1.10 dst=8.8.8.8 sport=50000 dport=443 packets=10 bytes=1000 src=8.8.8.8 dst=192.168.1.10 sport=443 dport=50000 packets=12 bytes=9000 [ASSURED] mark=0 use=1"

	flow, err := ParseLine(line)
	if err != nil {
		t.Fatal(err)
	}
	if flow.Protocol != "tcp" {
		t.Fatalf("expected tcp, got %s", flow.Protocol)
	}
	if flow.Original.SrcIP.String() != "192.168.1.10" || flow.Original.DstIP.String() != "8.8.8.8" {
		t.Fatalf("unexpected original tuple: %+v", flow.Original)
	}
	if flow.Counters.OriginalBytes != 1000 || flow.Counters.ReplyBytes != 9000 {
		t.Fatalf("unexpected counters: %+v", flow.Counters)
	}
}

func TestParseReader(t *testing.T) {
	input := strings.NewReader("ipv4 2 udp 17 10 src=192.168.1.10 dst=1.1.1.1 sport=53000 dport=53 packets=1 bytes=70 src=1.1.1.1 dst=192.168.1.10 sport=53 dport=53000 packets=1 bytes=120 mark=0 use=1\n")

	flows, err := ParseReader(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
}

func TestParseLineRequiresByteCounters(t *testing.T) {
	_, err := ParseLine("ipv4 2 tcp 6 20 SYN_SENT src=192.168.1.10 dst=8.8.8.8 sport=1 dport=443")
	if err == nil {
		t.Fatal("expected missing byte accounting error")
	}
}
