package alerts

import "testing"

func TestEvaluateDeviceDailyThreshold(t *testing.T) {
	alerts := Evaluate(Rule{Name: "device daily Internet > 2 GB", ThresholdBytes: 2_000_000_000}, Usage{
		DeviceID:   "dev_phone",
		DeviceName: "Achuthan Phone",
		TotalBytes: 2_400_000_000,
	})
	if len(alerts) != 1 {
		t.Fatalf("expected alert, got %d", len(alerts))
	}
}

func TestEvaluateCategoryAwareThresholdExcludesDownloads(t *testing.T) {
	alerts := Evaluate(Rule{
		Name:           "social/video daily usage",
		ThresholdBytes: 2_000_000_000,
		Included:       []string{"social", "video"},
		Excluded:       []string{"downloads", "software_updates"},
		UnknownPolicy:  UnknownTrafficExclude,
	}, Usage{
		DeviceID:   "dev_laptop",
		TotalBytes: 10_000_000_000,
		CategoryBytes: map[string]uint64{
			"downloads": 9_500_000_000,
			"video":     300_000_000,
		},
		UnknownBytes: 200_000_000,
	})
	if len(alerts) != 0 {
		t.Fatalf("download-heavy usage should not trigger social/video alert: %+v", alerts)
	}
}

func TestEvaluateCanIncludeUnknownTraffic(t *testing.T) {
	alerts := Evaluate(Rule{
		Name:           "unknown usage",
		ThresholdBytes: 1_000_000_000,
		UnknownPolicy:  UnknownTrafficInclude,
	}, Usage{
		DeviceID:     "dev_unknown",
		TotalBytes:   0,
		UnknownBytes: 1_200_000_000,
	})
	if len(alerts) != 1 {
		t.Fatalf("expected unknown traffic alert, got %d", len(alerts))
	}
}
