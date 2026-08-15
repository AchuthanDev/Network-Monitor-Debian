package domain

import (
	"net/netip"
	"time"
)

type Category string

const (
	CategoryVideoStreaming Category = "video_streaming"
	CategorySocialMedia    Category = "social_media"
	CategoryDownloads      Category = "downloads"
	CategorySoftwareUpdate Category = "software_update"
	CategoryGeneralWeb     Category = "general_web"
	CategoryDNS            Category = "dns"
	CategoryQUIC           Category = "quic"
	CategoryOther          Category = "other"
	CategoryUnknownHTTPS   Category = "unknown_https"
)

type Confidence string

const (
	ConfidenceHigh    Confidence = "high"
	ConfidenceMedium  Confidence = "medium"
	ConfidenceLow     Confidence = "low"
	ConfidenceUnknown Confidence = "unknown"
)

type Flow struct {
	Timestamp       time.Time
	DeviceID        string
	SourceIP        netip.Addr
	DestinationIP   netip.Addr
	Protocol        string
	DestinationPort uint16
	Domain          string
	DNSEvidence     []DNSEvidence
	SNIServerName   string
}

type DNSEvidence struct {
	Query      string
	ResolvedIP netip.Addr
	ObservedAt time.Time
	Source     string
}

type Classification struct {
	Service    string     `json:"service"`
	Category   Category   `json:"category"`
	Confidence Confidence `json:"confidence"`
	Evidence   []string   `json:"evidence"`
}

func UnknownHTTPS() Classification {
	return Classification{
		Service:    "Encrypted HTTPS / Unknown",
		Category:   CategoryUnknownHTTPS,
		Confidence: ConfidenceUnknown,
		Evidence:   []string{"metadata_only:no_dns_or_sni_match"},
	}
}
