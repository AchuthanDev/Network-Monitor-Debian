package alerts

import "strings"

type UnknownTrafficPolicy string

const (
	UnknownTrafficInclude UnknownTrafficPolicy = "include"
	UnknownTrafficExclude UnknownTrafficPolicy = "exclude"
)

type Rule struct {
	Name           string
	ThresholdBytes uint64
	Included       []string
	Excluded       []string
	UnknownPolicy  UnknownTrafficPolicy
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
}

func Evaluate(rule Rule, usage Usage) []Alert {
	bytes := matchedBytes(rule, usage)
	if bytes <= rule.ThresholdBytes {
		return nil
	}
	name := usage.DeviceName
	if name == "" {
		name = usage.DeviceID
	}
	return []Alert{{
		RuleName: rule.Name,
		DeviceID: usage.DeviceID,
		Message:  name + " exceeded " + rule.Name,
		Bytes:    bytes,
	}}
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

func contains(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(value, needle) {
			return true
		}
	}
	return false
}
