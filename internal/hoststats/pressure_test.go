package hoststats

import (
	"testing"

	"github.com/pterodactyl/wings/config"
)

func testMonitorConfig() config.HostMonitor {
	return config.HostMonitor{
		Enabled:           true,
		Interval:          2,
		WarningPercent:    85,
		CriticalPercent:   95,
		SamplesToTrigger:  3,
		HysteresisPercent: 5,
	}
}

func TestTrackerRequiresConsecutiveSamplesToRaiseLevel(t *testing.T) {
	tr := newTracker(testMonitorConfig(), config.HostMonitorThreshold{})

	for i := 0; i < 2; i++ {
		if got := tr.Push(90); got != LevelOK {
			t.Fatalf("sample %d: expected level to stay ok, got %q", i+1, got)
		}
	}
	if got := tr.Push(90); got != LevelWarning {
		t.Fatalf("expected the third sample to raise the level to warning, got %q", got)
	}
}

func TestTrackerResetsStreakWhenValueFallsBack(t *testing.T) {
	tr := newTracker(testMonitorConfig(), config.HostMonitorThreshold{})

	tr.Push(90)
	tr.Push(90)
	tr.Push(10)
	if got := tr.Push(90); got != LevelOK {
		t.Fatalf("expected the streak to restart after a low sample, got %q", got)
	}
}

func TestTrackerAppliesHysteresisWhenLoweringLevel(t *testing.T) {
	tr := newTracker(testMonitorConfig(), config.HostMonitorThreshold{})

	for i := 0; i < 3; i++ {
		tr.Push(90)
	}
	if tr.Level() != LevelWarning {
		t.Fatalf("expected setup to leave the tracker at warning, got %q", tr.Level())
	}

	// 82 is under the 85 warning threshold but still inside the 5 point margin.
	if got := tr.Push(82); got != LevelWarning {
		t.Fatalf("expected the level to hold inside the hysteresis margin, got %q", got)
	}
	if got := tr.Push(79); got != LevelOK {
		t.Fatalf("expected the level to drop below the hysteresis margin, got %q", got)
	}
}

func TestTrackerDropsFromCriticalToWarning(t *testing.T) {
	tr := newTracker(testMonitorConfig(), config.HostMonitorThreshold{})

	for i := 0; i < 3; i++ {
		tr.Push(99)
	}
	if tr.Level() != LevelCritical {
		t.Fatalf("expected setup to leave the tracker at critical, got %q", tr.Level())
	}

	if got := tr.Push(88); got != LevelWarning {
		t.Fatalf("expected the level to fall back to warning, got %q", got)
	}
}

func TestTrackerUsesPerResourceOverride(t *testing.T) {
	tr := newTracker(testMonitorConfig(), config.HostMonitorThreshold{WarningPercent: 90, CriticalPercent: 98})

	for i := 0; i < 3; i++ {
		tr.Push(88)
	}
	if got := tr.Level(); got != LevelOK {
		t.Fatalf("expected the override to keep the level at ok, got %q", got)
	}

	for i := 0; i < 3; i++ {
		tr.Push(92)
	}
	if got := tr.Level(); got != LevelWarning {
		t.Fatalf("expected the override warning threshold to apply, got %q", got)
	}
}

func TestHighestReturnsMostSevereLevel(t *testing.T) {
	cases := []struct {
		name     string
		levels   []Level
		expected Level
	}{
		{name: "all ok", levels: []Level{LevelOK, LevelOK, LevelOK}, expected: LevelOK},
		{name: "one warning", levels: []Level{LevelOK, LevelWarning, LevelOK}, expected: LevelWarning},
		{name: "critical wins", levels: []Level{LevelWarning, LevelCritical, LevelOK}, expected: LevelCritical},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := highest(tc.levels...); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}
