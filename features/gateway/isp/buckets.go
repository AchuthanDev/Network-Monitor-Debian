package isp

import (
	"fmt"
	"time"
)

type Period string

const (
	PeriodFreeNight Period = "free_night"
	PeriodAnytime   Period = "anytime"
)

type WindowConfig struct {
	Timezone  string `json:"timezone"`
	FreeStart string `json:"free_start"`
	FreeEnd   string `json:"free_end"`
}

func DefaultWindowConfig() WindowConfig {
	return WindowConfig{
		Timezone:  "Asia/Colombo",
		FreeStart: "00:00",
		FreeEnd:   "07:00",
	}
}

func (c WindowConfig) PeriodAt(observedAt time.Time) (Period, error) {
	location, err := time.LoadLocation(c.Timezone)
	if err != nil {
		return "", fmt.Errorf("load timezone %q: %w", c.Timezone, err)
	}
	start, err := parseClock(c.FreeStart)
	if err != nil {
		return "", err
	}
	end, err := parseClock(c.FreeEnd)
	if err != nil {
		return "", err
	}

	local := observedAt.In(location)
	second := local.Hour()*3600 + local.Minute()*60 + local.Second()
	if inWindow(second, start, end) {
		return PeriodFreeNight, nil
	}
	return PeriodAnytime, nil
}

func parseClock(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, fmt.Errorf("parse clock %q: %w", value, err)
	}
	return parsed.Hour()*3600 + parsed.Minute()*60, nil
}

func inWindow(second int, start int, end int) bool {
	if start == end {
		return false
	}
	if start < end {
		return second >= start && second < end
	}
	return second >= start || second < end
}
