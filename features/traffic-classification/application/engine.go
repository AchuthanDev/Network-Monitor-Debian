package application

import "github.com/AchuthanDev/Network-Monitor-Debian/features/traffic-classification/domain"

type Provider interface {
	Name() string
	Classify(flow domain.Flow) (domain.Classification, bool)
}

type Engine struct {
	providers []Provider
}

func NewEngine(providers ...Provider) Engine {
	return Engine{providers: providers}
}

func (e Engine) Classify(flow domain.Flow) domain.Classification {
	for _, provider := range e.providers {
		result, ok := provider.Classify(flow)
		if ok {
			return result
		}
	}
	if flow.Protocol == "udp" && flow.DestinationPort == 443 {
		return domain.Classification{
			Service:    "QUIC",
			Category:   domain.CategoryQUIC,
			Confidence: domain.ConfidenceLow,
			Evidence:   []string{"protocol:udp/443"},
		}
	}
	if flow.DestinationPort == 53 {
		return domain.Classification{
			Service:    "DNS",
			Category:   domain.CategoryDNS,
			Confidence: domain.ConfidenceHigh,
			Evidence:   []string{"port:53"},
		}
	}
	if flow.DestinationPort == 80 {
		return domain.Classification{
			Service:    "General HTTP",
			Category:   domain.CategoryGeneralWeb,
			Confidence: domain.ConfidenceLow,
			Evidence:   []string{"port:80"},
		}
	}
	if flow.DestinationPort == 443 {
		return domain.UnknownHTTPS()
	}
	return domain.Classification{
		Service:    "Other",
		Category:   domain.CategoryOther,
		Confidence: domain.ConfidenceUnknown,
		Evidence:   []string{"metadata_only:no_provider_match"},
	}
}
