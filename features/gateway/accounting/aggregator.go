package accounting

import (
	"context"
	"time"

	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/devices"
	"github.com/AchuthanDev/Network-Monitor-Debian/features/gateway/isp"
)

type MemoryAggregator struct {
	window isp.WindowConfig
	items  map[aggregateKey]AggregateDelta
}

type aggregateKey struct {
	minute       time.Time
	deviceID     string
	trafficClass TrafficClass
	ispPeriod    string
	category     string
	service      string
	confidence   ClassificationConfidence
}

func NewMemoryAggregator(window isp.WindowConfig) *MemoryAggregator {
	return &MemoryAggregator{
		window: window,
		items:  map[aggregateKey]AggregateDelta{},
	}
}

func (a *MemoryAggregator) Add(_ context.Context, event TrafficEvent, device devices.Device, classification TrafficClassification) error {
	period, err := a.window.PeriodAt(event.Timestamp)
	if err != nil {
		return err
	}
	category := classification.Category
	if category == "" {
		category = "unknown"
	}
	confidence := classification.Confidence
	if confidence == "" {
		confidence = ConfidenceUnknown
	}
	trafficClass := classification.Class
	if trafficClass == "" {
		trafficClass = event.TrafficClass
	}
	if trafficClass == "" {
		trafficClass = TrafficUnknown
	}

	minute := event.Timestamp.UTC().Truncate(time.Minute)
	key := aggregateKey{
		minute:       minute,
		deviceID:     device.ID,
		trafficClass: trafficClass,
		ispPeriod:    string(period),
		category:     category,
		service:      classification.Service,
		confidence:   confidence,
	}
	item := a.items[key]
	item.Timestamp = minute
	item.DeviceID = device.ID
	item.TrafficClass = trafficClass
	item.ISPPeriod = string(period)
	item.Category = category
	item.Service = classification.Service
	item.Confidence = confidence

	switch event.Direction {
	case DirectionDownload:
		item.DownloadBytes += event.Bytes
	case DirectionUpload:
		item.UploadBytes += event.Bytes
	}
	a.items[key] = item
	return nil
}

func (a *MemoryAggregator) Flush(context.Context) ([]AggregateDelta, error) {
	result := make([]AggregateDelta, 0, len(a.items))
	for key, item := range a.items {
		result = append(result, item)
		delete(a.items, key)
	}
	return result, nil
}
