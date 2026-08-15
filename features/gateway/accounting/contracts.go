package accounting

import (
	"context"
	"net/netip"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/devices"
)

type Direction string

const (
	DirectionDownload Direction = "download"
	DirectionUpload   Direction = "upload"
)

type TrafficClass string

const (
	TrafficInternet       TrafficClass = "internet"
	TrafficLAN            TrafficClass = "lan"
	TrafficServerLocal    TrafficClass = "server_local"
	TrafficDockerInternal TrafficClass = "docker_internal"
	TrafficUnknown        TrafficClass = "unknown"
)

type ClassificationConfidence string

const (
	ConfidenceHigh    ClassificationConfidence = "high"
	ConfidenceMedium  ClassificationConfidence = "medium"
	ConfidenceLow     ClassificationConfidence = "low"
	ConfidenceUnknown ClassificationConfidence = "unknown"
)

type TrafficEvent struct {
	Timestamp     time.Time
	SourceIP      netip.Addr
	SourceMAC     string
	DestinationIP netip.Addr
	SourcePort    uint16
	DestPort      uint16
	Protocol      string
	Direction     Direction
	Bytes         uint64
	Interface     string
	TrafficClass  TrafficClass
	PreNAT        bool
	FlowID        string
}

type TrafficClassification struct {
	Class      TrafficClass
	Category   string
	Service    string
	Confidence ClassificationConfidence
	Reason     string
}

type Destination struct {
	IP         netip.Addr
	Hostname   string
	ASN        string
	Provider   string
	Confidence ClassificationConfidence
}

type AggregateDelta struct {
	Timestamp     time.Time
	DeviceID      string
	TrafficClass  TrafficClass
	DownloadBytes uint64
	UploadBytes   uint64
	ISPPeriod     string
	Category      string
	Service       string
	Confidence    ClassificationConfidence
}

type TrafficEventCollector interface {
	Collect(ctx context.Context) (<-chan TrafficEvent, error)
}

type DeviceResolver interface {
	Resolve(ctx context.Context, event TrafficEvent) (devices.Device, error)
}

type TrafficClassifier interface {
	Classify(ctx context.Context, event TrafficEvent) (TrafficClassification, error)
}

type TrafficAggregator interface {
	Add(ctx context.Context, event TrafficEvent, device devices.Device, classification TrafficClassification) error
	Flush(ctx context.Context) ([]AggregateDelta, error)
}

type DestinationResolver interface {
	Resolve(ctx context.Context, event TrafficEvent) (Destination, error)
}
