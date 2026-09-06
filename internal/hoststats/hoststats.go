// Package hoststats collects utilization data for the machine Wings itself is
// running on. The data is exposed to the Panel over an HTTP endpoint and pushed
// to administrators over the server websocket so that a saturated host can be
// spotted even when the per container graphs look healthy.
package hoststats

import "time"

// Level describes how much pressure a resource, or the host as a whole, is
// currently under.
type Level string

const (
	LevelOK       Level = "ok"
	LevelWarning  Level = "warning"
	LevelCritical Level = "critical"
)

// weight returns a comparable ranking for a level so that the highest level of
// a set of resources can be resolved.
func (l Level) weight() int {
	switch l {
	case LevelCritical:
		return 2
	case LevelWarning:
		return 1
	default:
		return 0
	}
}

// Load holds the system load averages over the three usual windows.
type Load struct {
	One     float64 `json:"1"`
	Five    float64 `json:"5"`
	Fifteen float64 `json:"15"`
}

// CPU holds the processor utilization of the host.
type CPU struct {
	Threads int     `json:"threads"`
	Percent float64 `json:"percent"`
	Load    Load    `json:"load"`
	// LoadPercent is the one minute load average expressed as a percentage of
	// the number of available threads.
	LoadPercent float64 `json:"load_percent"`
}

// Memory holds the physical memory utilization of the host.
type Memory struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	Percent        float64 `json:"percent"`
}

// Swap holds the swap utilization of the host. It is reported for context only
// and never contributes to the pressure level.
type Swap struct {
	TotalBytes uint64  `json:"total_bytes"`
	UsedBytes  uint64  `json:"used_bytes"`
	Percent    float64 `json:"percent"`
}

// Disk holds the utilization of a single filesystem. Several Wings directories
// frequently live on the same filesystem, in which case their labels are merged
// into a single entry.
type Disk struct {
	Labels     []string `json:"labels"`
	Path       string   `json:"path"`
	TotalBytes uint64   `json:"total_bytes"`
	UsedBytes  uint64   `json:"used_bytes"`
	Percent    float64  `json:"percent"`
}

// Servers holds aggregated information about the servers running on this host,
// including how much of the host has been handed out to them.
type Servers struct {
	Total     int `json:"total"`
	Running   int `json:"running"`
	Unlimited int `json:"unlimited"`

	MemoryBytes uint64  `json:"memory_bytes"`
	CpuAbsolute float64 `json:"cpu_absolute"`

	MemoryAllocatedBytes int64 `json:"memory_allocated_bytes"`
	CpuAllocatedPercent  int64 `json:"cpu_allocated_percent"`
}

// Thresholds reports the global percentages that were used to resolve the
// levels in a snapshot.
type Thresholds struct {
	WarningPercent  float64 `json:"warning_percent"`
	CriticalPercent float64 `json:"critical_percent"`
}

// Pressure describes the resolved level of the host and of each individual
// resource that contributes to it.
type Pressure struct {
	Level      Level            `json:"level"`
	Resources  map[string]Level `json:"resources"`
	Thresholds Thresholds       `json:"thresholds"`
}

// Snapshot is a single sample of the host state.
type Snapshot struct {
	Timestamp       time.Time `json:"timestamp"`
	IntervalSeconds int       `json:"interval_seconds"`

	CPU     CPU      `json:"cpu"`
	Memory  Memory   `json:"memory"`
	Swap    Swap     `json:"swap"`
	Disks   []Disk   `json:"disks"`
	Servers Servers  `json:"servers"`
	Pressure Pressure `json:"pressure"`
}

// PressureChange is the payload published whenever the overall level of the
// host changes. It exists so that notification transports can be attached
// without having to diff every single sample.
type PressureChange struct {
	Previous Level    `json:"previous"`
	Current  Level    `json:"current"`
	Snapshot Snapshot `json:"snapshot"`
}
