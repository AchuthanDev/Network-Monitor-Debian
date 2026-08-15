package alerts

import (
	"testing"
	"time"
)

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

func TestPhoneYouTubeTriggersSocialVideoAlert(t *testing.T) {
	alerts := Evaluate(Rule{
		Name:           "social/video daily usage",
		ThresholdBytes: 2_000_000_000,
		Included:       []string{"social_media", "video_streaming"},
		UnknownPolicy:  UnknownTrafficExclude,
	}, Usage{
		DeviceID: "dev_phone",
		CategoryBytes: map[string]uint64{
			"video_streaming": 2_500_000_000,
		},
	})
	if len(alerts) != 1 {
		t.Fatalf("expected social/video alert, got %d", len(alerts))
	}
}

func TestKnownDownloadDoesNotTriggerSocialVideoAlert(t *testing.T) {
	alerts := Evaluate(Rule{
		Name:           "social/video daily usage",
		ThresholdBytes: 2_000_000_000,
		Included:       []string{"social_media", "video_streaming"},
		Excluded:       []string{"downloads"},
		UnknownPolicy:  UnknownTrafficExclude,
	}, Usage{
		DeviceID: "dev_laptop",
		CategoryBytes: map[string]uint64{
			"downloads": 10_000_000_000,
		},
	})
	if len(alerts) != 0 {
		t.Fatalf("known download should not trigger social/video alert: %+v", alerts)
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

func TestUnknownHTTPSTriggersDedicatedUnknownRule(t *testing.T) {
	alerts := Evaluate(Rule{
		Name:           "unknown HTTPS daily usage",
		Metric:         "unknown",
		ThresholdBytes: 2_000_000_000,
	}, Usage{DeviceID: "dev_phone", UnknownBytes: 3_000_000_000})
	if len(alerts) != 1 {
		t.Fatalf("expected unknown HTTPS alert, got %d", len(alerts))
	}
}

func TestBurstAndUploadSpikeRules(t *testing.T) {
	burst := Evaluate(Rule{Name: "1 GB transferred within 10 minutes", Metric: "burst_10m", ThresholdBytes: 1_000_000_000}, Usage{
		DeviceID:      "dev_phone",
		BurstBytes10m: 1_200_000_000,
	})
	if len(burst) != 1 {
		t.Fatalf("expected burst alert, got %d", len(burst))
	}
	upload := Evaluate(Rule{Name: "upload spike", Metric: "upload", ThresholdBytes: 500_000_000}, Usage{
		DeviceID:    "dev_laptop",
		UploadBytes: 650_000_000,
	})
	if len(upload) != 1 {
		t.Fatalf("expected upload alert, got %d", len(upload))
	}
}

func TestDeduplicatorSuppressesRepeatedTierWithinCooldown(t *testing.T) {
	dedupe := NewDeduplicator()
	rule := Rule{
		Name:           "social/video daily usage",
		Thresholds:     []uint64{2_000_000_000, 5_000_000_000},
		Included:       []string{"social_media", "video_streaming"},
		Cooldown:       time.Hour,
		UnknownPolicy:  UnknownTrafficExclude,
		ThresholdBytes: 2_000_000_000,
	}
	usage := Usage{
		DeviceID: "dev_phone",
		CategoryBytes: map[string]uint64{
			"video_streaming": 2_400_000_000,
		},
	}
	now := time.Date(2026, 8, 15, 14, 0, 0, 0, time.UTC)
	if got := dedupe.Evaluate(rule, usage, now); len(got) != 1 {
		t.Fatalf("expected first alert, got %d", len(got))
	}
	if got := dedupe.Evaluate(rule, usage, now.Add(time.Minute)); len(got) != 0 {
		t.Fatalf("expected duplicate to be suppressed, got %+v", got)
	}
	usage.CategoryBytes["video_streaming"] = 5_500_000_000
	if got := dedupe.Evaluate(rule, usage, now.Add(2*time.Minute)); len(got) != 1 || got[0].Tier != 5_000_000_000 {
		t.Fatalf("expected higher tier alert, got %+v", got)
	}
}
