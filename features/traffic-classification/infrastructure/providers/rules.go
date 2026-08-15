package providers

import (
	"strings"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"
)

type DomainRule struct {
	Service  string
	Category domain.Category
	Suffixes []string
}

type DomainProvider struct {
	name  string
	rules []DomainRule
}

func NewDomainProvider(name string, rules []DomainRule) DomainProvider {
	return DomainProvider{name: name, rules: rules}
}

func (p DomainProvider) Name() string {
	return p.name
}

func (p DomainProvider) Classify(flow domain.Flow) (domain.Classification, bool) {
	candidates := make([]string, 0, 1+len(flow.DNSEvidence))
	if flow.Domain != "" {
		candidates = append(candidates, flow.Domain)
	}
	if flow.SNIServerName != "" {
		candidates = append(candidates, flow.SNIServerName)
	}
	for _, evidence := range flow.DNSEvidence {
		if evidence.Query != "" {
			candidates = append(candidates, evidence.Query)
		}
	}

	for _, candidate := range candidates {
		normalized := normalizeDomain(candidate)
		for _, rule := range p.rules {
			for _, suffix := range rule.Suffixes {
				if domainMatches(normalized, suffix) {
					return domain.Classification{
						Service:    rule.Service,
						Category:   rule.Category,
						Confidence: confidenceForCandidate(flow, candidate),
						Evidence:   []string{"dns_or_sni:" + normalized},
					}, true
				}
			}
		}
	}
	return domain.Classification{}, false
}

func confidenceForCandidate(flow domain.Flow, candidate string) domain.Confidence {
	for _, evidence := range flow.DNSEvidence {
		if strings.EqualFold(evidence.Query, candidate) && evidence.ResolvedIP.IsValid() && evidence.ResolvedIP == flow.DestinationIP {
			return domain.ConfidenceHigh
		}
	}
	if strings.EqualFold(flow.SNIServerName, candidate) && flow.SNIServerName != "" {
		return domain.ConfidenceMedium
	}
	return domain.ConfidenceMedium
}

func normalizeDomain(value string) string {
	return strings.Trim(strings.ToLower(value), ".")
}

func domainMatches(value string, suffix string) bool {
	suffix = normalizeDomain(suffix)
	return value == suffix || strings.HasSuffix(value, "."+suffix)
}
