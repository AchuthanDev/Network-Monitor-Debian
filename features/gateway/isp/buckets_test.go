package isp

import (
	"testing"
	"time"
)

func TestDefaultWindowBoundaries(t *testing.T) {
	cfg := DefaultWindowConfig()
	cases := []struct {
		name string
		at   string
		want Period
	}{
		{name: "before midnight", at: "2026-08-14T23:59:59+05:30", want: PeriodAnytime},
		{name: "start inclusive", at: "2026-08-15T00:00:00+05:30", want: PeriodFreeNight},
		{name: "end minus second", at: "2026-08-15T06:59:59+05:30", want: PeriodFreeNight},
		{name: "end exclusive", at: "2026-08-15T07:00:00+05:30", want: PeriodAnytime},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cfg.PeriodAt(mustTime(t, tc.at))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestCrossMidnightWindow(t *testing.T) {
	cfg := WindowConfig{Timezone: "Asia/Colombo", FreeStart: "22:00", FreeEnd: "02:00"}
	cases := []struct {
		at   string
		want Period
	}{
		{at: "2026-08-15T21:59:59+05:30", want: PeriodAnytime},
		{at: "2026-08-15T22:00:00+05:30", want: PeriodFreeNight},
		{at: "2026-08-16T01:59:59+05:30", want: PeriodFreeNight},
		{at: "2026-08-16T02:00:00+05:30", want: PeriodAnytime},
	}

	for _, tc := range cases {
		got, err := cfg.PeriodAt(mustTime(t, tc.at))
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("%s expected %s, got %s", tc.at, tc.want, got)
		}
	}
}

func TestTimezoneConversion(t *testing.T) {
	cfg := DefaultWindowConfig()

	got, err := cfg.PeriodAt(mustTime(t, "2026-08-14T18:30:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got != PeriodFreeNight {
		t.Fatalf("18:30 UTC should be midnight in Asia/Colombo and free/night, got %s", got)
	}
}

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
