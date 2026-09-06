package hoststats

import "github.com/pterodactyl/wings/server"

// ServerUsage is the contribution of a single server to the host aggregate. It
// is kept free of any Wings types so that the aggregation can be exercised on
// its own.
type ServerUsage struct {
	Running     bool
	MemoryBytes uint64
	CpuAbsolute float64

	// MemoryLimitMegabytes and CpuLimitPercent are the limits the Panel handed
	// to this server. A value of zero means the resource is unlimited.
	MemoryLimitMegabytes int64
	CpuLimitPercent      int64
}

// UsageProvider returns the current usage of every server on this host.
type UsageProvider func() []ServerUsage

// FromManager adapts a server manager into a UsageProvider.
func FromManager(m *server.Manager) UsageProvider {
	return func() []ServerUsage {
		all := m.All()
		out := make([]ServerUsage, 0, len(all))
		for _, s := range all {
			proc := s.Proc()
			out = append(out, ServerUsage{
				Running:              s.IsRunning(),
				MemoryBytes:          proc.Memory,
				CpuAbsolute:          proc.CpuAbsolute,
				MemoryLimitMegabytes: s.Config().Build.MemoryLimit,
				CpuLimitPercent:      s.Config().Build.CpuLimit,
			})
		}
		return out
	}
}

// aggregateServers folds the per server usage into the totals reported in a
// snapshot. Servers without a limit are counted separately instead of being
// added to the allocation totals, where they would read as zero.
func aggregateServers(usage []ServerUsage) Servers {
	out := Servers{Total: len(usage)}
	for _, u := range usage {
		if u.Running {
			out.Running++
		}

		out.MemoryBytes += u.MemoryBytes
		out.CpuAbsolute += u.CpuAbsolute

		if u.MemoryLimitMegabytes == 0 || u.CpuLimitPercent == 0 {
			out.Unlimited++
		}
		if u.MemoryLimitMegabytes > 0 {
			out.MemoryAllocatedBytes += u.MemoryLimitMegabytes * 1024 * 1024
		}
		if u.CpuLimitPercent > 0 {
			out.CpuAllocatedPercent += u.CpuLimitPercent
		}
	}
	return out
}
