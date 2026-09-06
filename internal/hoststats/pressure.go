package hoststats

import "github.com/pterodactyl/wings/config"

// tracker resolves the level of a single resource over time. Raising a level
// requires the value to stay above the threshold for a number of consecutive
// samples so that a short spike does not immediately alarm an administrator.
// Lowering it requires the value to drop a configurable number of percentage
// points below the threshold so that a value hovering right on the line does
// not flap between two levels.
type tracker struct {
	warning  float64
	critical float64

	samplesToTrigger int
	hysteresis       float64

	level Level
	// candidate is the level the current run of samples is pushing towards, and
	// streak is how many consecutive samples have agreed on it.
	candidate Level
	streak    int
}

func newTracker(cfg config.HostMonitor, override config.HostMonitorThreshold) *tracker {
	samples := cfg.SamplesToTrigger
	if samples < 1 {
		samples = 1
	}

	return &tracker{
		warning:          cfg.WarningFor(override),
		critical:         cfg.CriticalFor(override),
		samplesToTrigger: samples,
		hysteresis:       cfg.HysteresisPercent,
		level:            LevelOK,
		candidate:        LevelOK,
	}
}

// levelFor returns the level a raw value maps to, taking the current level into
// account so that hysteresis is only applied when dropping back down.
func (t *tracker) levelFor(value float64) Level {
	critical, warning := t.critical, t.warning
	// Only relax the threshold we are currently sitting at or above, otherwise a
	// host at 91% with a 5 point hysteresis would never reach "warning" again
	// after having been "critical".
	if t.level == LevelCritical {
		critical -= t.hysteresis
	}
	if t.level != LevelOK {
		warning -= t.hysteresis
	}

	if value >= critical {
		return LevelCritical
	}
	if value >= warning {
		return LevelWarning
	}
	return LevelOK
}

// Push feeds a new sample into the tracker and returns the level that should be
// reported for the resource.
func (t *tracker) Push(value float64) Level {
	target := t.levelFor(value)
	if target == t.level {
		t.candidate = target
		t.streak = 0
		return t.level
	}

	if target != t.candidate {
		t.candidate = target
		t.streak = 0
	}
	t.streak++

	// Dropping to a lower level is already guarded by the hysteresis margin, so
	// only raising a level has to wait for a run of consecutive samples.
	if target.weight() < t.level.weight() || t.streak >= t.samplesToTrigger {
		t.level = target
		t.streak = 0
	}

	return t.level
}

// Level returns the level currently being reported without feeding a sample.
func (t *tracker) Level() Level {
	return t.level
}

// highest returns the most severe of the given levels.
func highest(levels ...Level) Level {
	out := LevelOK
	for _, l := range levels {
		if l.weight() > out.weight() {
			out = l
		}
	}
	return out
}
