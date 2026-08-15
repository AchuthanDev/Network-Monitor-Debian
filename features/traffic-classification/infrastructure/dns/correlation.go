package dns

import (
	"net/netip"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"
)

type Query struct {
	ClientIP   netip.Addr
	Domain     string
	ResolvedIP netip.Addr
	ObservedAt time.Time
	Source     string
}

type Correlator struct {
	window  time.Duration
	queries []Query
}

func NewCorrelator(window time.Duration) *Correlator {
	return &Correlator{window: window}
}

func (c *Correlator) Add(query Query) {
	if query.ClientIP.IsValid() && query.ResolvedIP.IsValid() && query.Domain != "" {
		c.queries = append(c.queries, query)
	}
}

func (c *Correlator) Evidence(clientIP netip.Addr, destinationIP netip.Addr, at time.Time) []domain.DNSEvidence {
	var result []domain.DNSEvidence
	cutoff := at.Add(-c.window)
	for _, query := range c.queries {
		if query.ClientIP != clientIP || query.ResolvedIP != destinationIP {
			continue
		}
		if query.ObservedAt.Before(cutoff) || query.ObservedAt.After(at.Add(c.window)) {
			continue
		}
		result = append(result, domain.DNSEvidence{
			Query:      query.Domain,
			ResolvedIP: query.ResolvedIP,
			ObservedAt: query.ObservedAt,
			Source:     query.Source,
		})
	}
	return result
}
