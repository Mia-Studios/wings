package hoststats

import (
	"context"
	"time"

	"github.com/apex/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/pterodactyl/wings/config"
	"github.com/pterodactyl/wings/events"
	"github.com/pterodactyl/wings/system"
)

const (
	// StatsEvent carries a full snapshot on every sample.
	StatsEvent = "host stats"
	// PressureEvent is only published when the overall level of the host changes.
	PressureEvent = "host pressure"
)

// Sampler periodically collects the state of the host and publishes it on its
// event bus.
type Sampler struct {
	cfg      config.HostMonitor
	interval time.Duration

	usage UsageProvider
	// dockerRoot is the directory the Docker daemon stores its data in. It is
	// resolved once at boot because it cannot change while Wings is running.
	dockerRoot string

	bus      *events.Bus
	latest   *system.Atomic[*Snapshot]
	trackers map[string]*tracker
	level    Level
}

// New builds a sampler from the given configuration. The returned sampler does
// not collect anything until Run is called.
func New(cfg config.HostMonitor, usage UsageProvider, dockerRoot string) *Sampler {
	interval := time.Duration(cfg.Interval) * time.Second
	if interval <= 0 {
		interval = time.Second * 2
	}

	return &Sampler{
		cfg:        cfg,
		interval:   interval,
		usage:      usage,
		dockerRoot: dockerRoot,
		bus:        events.NewBus(),
		latest:     system.NewAtomic[*Snapshot](nil),
		trackers: map[string]*tracker{
			"cpu":    newTracker(cfg, cfg.Thresholds.Cpu),
			"memory": newTracker(cfg, cfg.Thresholds.Memory),
			"disk":   newTracker(cfg, cfg.Thresholds.Disk),
		},
		level: LevelOK,
	}
}

// Bus returns the event bus snapshots are published on.
func (s *Sampler) Bus() *events.Bus {
	return s.bus
}

// Latest returns the most recent snapshot, or nil when no sample has been
// collected yet.
func (s *Sampler) Latest() *Snapshot {
	return s.latest.Load()
}

// Run collects a sample every interval until the context is canceled. It blocks
// and is expected to be called in its own routine.
func (s *Sampler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.Collect()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Collect()
		}
	}
}

// Collect takes a single sample and publishes it. Run calls this on every tick.
func (s *Sampler) Collect() {
	snapshot, err := s.sample()
	if err != nil {
		log.WithField("subsystem", "host_monitor").WithField("error", err).
			Warn("failed to collect host utilization sample")
		return
	}

	previous := s.level
	s.level = snapshot.Pressure.Level

	s.latest.Store(snapshot)
	s.bus.Publish(StatsEvent, snapshot)

	if previous != s.level {
		s.bus.Publish(PressureEvent, PressureChange{
			Previous: previous,
			Current:  s.level,
			Snapshot: *snapshot,
		})
	}
}

// sample reads the current state of the host and resolves the pressure levels
// that go with it.
func (s *Sampler) sample() (*Snapshot, error) {
	out := &Snapshot{
		Timestamp:       time.Now().UTC(),
		IntervalSeconds: int(s.interval.Seconds()),
	}

	threads, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}
	out.CPU.Threads = threads

	// A zero interval reads the utilization since the previous call, which is
	// exactly the window between two samples.
	if percents, err := cpu.Percent(0, false); err == nil && len(percents) > 0 {
		out.CPU.Percent = percents[0]
	}

	if avg, err := load.Avg(); err == nil {
		out.CPU.Load = Load{One: avg.Load1, Five: avg.Load5, Fifteen: avg.Load15}
		if threads > 0 {
			out.CPU.LoadPercent = avg.Load1 / float64(threads) * 100
		}
	}

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	out.Memory = Memory{
		TotalBytes:     vm.Total,
		AvailableBytes: vm.Available,
		// The used value reported by the kernel excludes reclaimable memory, which
		// is not what an administrator looking for pressure cares about.
		UsedBytes: vm.Total - vm.Available,
	}
	if vm.Total > 0 {
		out.Memory.Percent = float64(vm.Total-vm.Available) / float64(vm.Total) * 100
	}

	if sm, err := mem.SwapMemory(); err == nil {
		out.Swap = Swap{TotalBytes: sm.Total, UsedBytes: sm.Used, Percent: sm.UsedPercent}
	}

	out.Disks = collectDisks(s.mounts())
	out.Servers = aggregateServers(s.usage())

	// A host can be starved by scheduling pressure long before the sampled CPU
	// utilization reflects it, so the worse of the two is what counts.
	cpuPressure := out.CPU.Percent
	if out.CPU.LoadPercent > cpuPressure {
		cpuPressure = out.CPU.LoadPercent
	}

	resources := map[string]Level{
		"cpu":    s.trackers["cpu"].Push(cpuPressure),
		"memory": s.trackers["memory"].Push(out.Memory.Percent),
		"disk":   s.trackers["disk"].Push(worstDiskPercent(out.Disks)),
	}

	out.Pressure = Pressure{
		Level:     highest(resources["cpu"], resources["memory"], resources["disk"]),
		Resources: resources,
		Thresholds: Thresholds{
			WarningPercent:  s.cfg.WarningPercent,
			CriticalPercent: s.cfg.CriticalPercent,
		},
	}

	return out, nil
}

// mounts returns the directories whose filesystem utilization is reported. The
// order matters: the first label of a merged entry is the one whose path is
// reported for it.
func (s *Sampler) mounts() []mount {
	cfg := config.Get().System
	return []mount{
		{label: "data", path: cfg.Data},
		{label: "backups", path: cfg.BackupDirectory},
		{label: "docker", path: s.dockerRoot},
		{label: "root", path: "/"},
	}
}

// instance holds the sampler started at boot so that the HTTP router and the
// websocket handlers can reach it without having to thread it through every
// call site. It is nil when the host monitor is disabled.
var instance = system.NewAtomic[*Sampler](nil)

// Set registers the sampler that the rest of Wings should read from.
func Set(s *Sampler) {
	instance.Store(s)
}

// Get returns the registered sampler, or nil when the host monitor is disabled.
func Get() *Sampler {
	return instance.Load()
}
