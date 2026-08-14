package conntrack

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

type Tuple struct {
	SrcIP   netip.Addr
	DstIP   netip.Addr
	SrcPort uint16
	DstPort uint16
}

type Counters struct {
	OriginalBytes uint64
	ReplyBytes    uint64
}

type Flow struct {
	Family   string
	Protocol string
	Original Tuple
	Reply    Tuple
	Counters Counters
}

func (f Flow) Key() string {
	return fmt.Sprintf("%s|%s|%s:%d>%s:%d|%s:%d>%s:%d",
		f.Family,
		f.Protocol,
		f.Original.SrcIP,
		f.Original.SrcPort,
		f.Original.DstIP,
		f.Original.DstPort,
		f.Reply.SrcIP,
		f.Reply.SrcPort,
		f.Reply.DstIP,
		f.Reply.DstPort,
	)
}

func ParseReader(reader io.Reader) ([]Flow, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var flows []Flow
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		flow, err := ParseLine(line)
		if err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return flows, nil
}

func ParseLine(line string) (Flow, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return Flow{}, fmt.Errorf("conntrack line too short")
	}

	flow := Flow{
		Family:   fields[0],
		Protocol: fields[2],
	}

	section := 0
	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}

		switch key {
		case "src":
			ip, err := netip.ParseAddr(value)
			if err != nil {
				return Flow{}, fmt.Errorf("parse src ip %q: %w", value, err)
			}
			if section == 0 {
				flow.Original.SrcIP = ip
			} else {
				flow.Reply.SrcIP = ip
			}
		case "dst":
			ip, err := netip.ParseAddr(value)
			if err != nil {
				return Flow{}, fmt.Errorf("parse dst ip %q: %w", value, err)
			}
			if section == 0 {
				flow.Original.DstIP = ip
			} else {
				flow.Reply.DstIP = ip
			}
		case "sport":
			port, err := parsePort(value)
			if err != nil {
				return Flow{}, err
			}
			if section == 0 {
				flow.Original.SrcPort = port
			} else {
				flow.Reply.SrcPort = port
			}
		case "dport":
			port, err := parsePort(value)
			if err != nil {
				return Flow{}, err
			}
			if section == 0 {
				flow.Original.DstPort = port
			} else {
				flow.Reply.DstPort = port
			}
		case "bytes":
			bytes, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return Flow{}, fmt.Errorf("parse bytes %q: %w", value, err)
			}
			if section == 0 {
				flow.Counters.OriginalBytes = bytes
				section = 1
			} else {
				flow.Counters.ReplyBytes = bytes
			}
		}
	}

	if !flow.Original.SrcIP.IsValid() || !flow.Original.DstIP.IsValid() {
		return Flow{}, fmt.Errorf("conntrack line missing original tuple")
	}
	if flow.Counters.OriginalBytes == 0 && flow.Counters.ReplyBytes == 0 && !strings.Contains(line, "bytes=") {
		return Flow{}, fmt.Errorf("conntrack byte accounting unavailable in line")
	}
	return flow, nil
}

func parsePort(value string) (uint16, error) {
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("parse port %q: %w", value, err)
	}
	return uint16(port), nil
}
