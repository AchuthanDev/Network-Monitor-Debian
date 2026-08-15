package pihole

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	dnscorrelation "github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/infrastructure/dns"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type Reader struct {
	DatabasePath string
	Runner       CommandRunner
}

func (r Reader) ReadSince(ctx context.Context, since time.Time, limit int) ([]dnscorrelation.Query, error) {
	if limit <= 0 {
		limit = 1000
	}
	dbPath := r.DatabasePath
	if dbPath == "" {
		dbPath = "/etc/pihole/pihole-FTL.db"
	}
	query := fmt.Sprintf(`SELECT timestamp, client, domain, COALESCE(reply_addr, '')
FROM queries
WHERE timestamp > %d
ORDER BY timestamp ASC
LIMIT %d;`, since.Unix(), limit)

	if r.Runner == nil {
		return nil, fmt.Errorf("pihole reader has no command runner")
	}
	output, err := r.Runner.Run(ctx, "pihole-FTL", "sqlite3", "-readonly", "-separator", "|", dbPath, query)
	if err != nil {
		return nil, err
	}
	return ParseFTLRows(output), nil
}

func ParseFTLRows(output []byte) []dnscorrelation.Query {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	result := make([]dnscorrelation.Query, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		timestamp, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		client, err := netip.ParseAddr(parts[1])
		if err != nil {
			continue
		}
		resolved, _ := netip.ParseAddr(parts[3])
		result = append(result, dnscorrelation.Query{
			ClientIP:   client,
			Domain:     parts[2],
			ResolvedIP: resolved,
			ObservedAt: time.Unix(timestamp, 0).UTC(),
			Source:     "pihole-ftl",
		})
	}
	return result
}
