package alerts

import (
	"fmt"
	"strings"
	"time"
)

type UnknownTrafficPolicy string

const (
	UnknownTrafficInclude UnknownTrafficPolicy = "include"
	UnknownTrafficExclude UnknownTrafficPolicy = "exclude"
)

type Rule struct {
	Name           string
	ThresholdBytes uint64
	Thresholds     []uint64
	Included       []string
	Excluded       []string
	UnknownPolicy  UnknownTrafficPolicy
	Cooldown       time.Duration
}

type Usage struct {
	DeviceID      string
	DeviceName    string
	TotalBytes    uint64
	AnytimeBytes  uint64
	UnknownBytes  uint64
	CategoryBytes map[string]uint64
}

type Alert struct {
	RuleName string `json:"rule_name"`
	DeviceID string `json:"device_id"`
	Message  string `json:"message"`
	Bytes    uint64 `json:"bytes"`
	Tier     uint64 `json:"tier_bytes"`
}

func Evaluate(rule Rule, usage Usage) []Alert {
	return EvaluateAt(rule, usage, time.Now())
}

type Deduplicator struct {
	sent map[string]time.Time
}

func NewDeduplicator() *Deduplicator {
	return &Deduplicator{sent: map[string]time.Time{}}
}

func (d *Deduplicator) Evaluate(rule Rule, usage Usage, now time.Time) []Alert {
	alerts := EvaluateAt(rule, usage, now)
	if len(alerts) == 0 {
		return nil
	}
	filtered := alerts[:0]
	for _, alert := range alerts {
		key := dedupeKey(alert)
		previous, ok := d.sent[key]
		if ok && rule.Cooldown > 0 && now.Sub(previous) < rule.Cooldown {
			continue
		}
		d.sent[key] = now
		filtered = append(filtered, alert)
	}
	return filtered
}

func EvaluateAt(rule Rule, usage Usage, _ time.Time) []Alert {
	bytes := matchedBytes(rule, usage)
	thresholds := rule.Thresholds
	if len(thresholds) == 0 {
		thresholds = []uint64{rule.ThresholdBytes}
	}
	name := usage.DeviceName
	if name == "" {
		name = usage.DeviceID
	}
	var alerts []Alert
	for _, threshold := range thresholds {
		if threshold == 0 || bytes <= threshold {
			continue
		}
		alerts = append(alerts, Alert{
			RuleName: rule.Name,
			DeviceID: usage.DeviceID,
			Message:  fmt.Sprintf("%s exceeded %s at %d bytes", name, rule.Name, threshold),
			Bytes:    bytes,
			Tier:     threshold,
		})
	}
	return alerts
}

func matchedBytes(rule Rule, usage Usage) uint64 {
	if len(rule.Included) == 0 && len(rule.Excluded) == 0 {
		if rule.UnknownPolicy == UnknownTrafficInclude {
			return usage.TotalBytes + usage.UnknownBytes
		}
		return usage.TotalBytes
	}
	var total uint64
	for category, bytes := range usage.CategoryBytes {
		if len(rule.Included) > 0 && !contains(rule.Included, category) {
			continue
		}
		if contains(rule.Excluded, category) {
			continue
		}
		total += bytes
	}
	if rule.UnknownPolicy == UnknownTrafficInclude && !contains(rule.Excluded, "unknown") {
		total += usage.UnknownBytes
	}
	return total
}

func dedupeKey(alert Alert) string {
	return fmt.Sprintf("%s|%s|%d", alert.RuleName, alert.DeviceID, alert.Tier)
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
