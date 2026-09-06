package hoststats

import "testing"

func TestAggregateServersSumsUsageAndAllocation(t *testing.T) {
	out := aggregateServers([]ServerUsage{
		{Running: true, MemoryBytes: 1024, CpuAbsolute: 12.5, MemoryLimitMegabytes: 1024, CpuLimitPercent: 100},
		{Running: false, MemoryBytes: 0, CpuAbsolute: 0, MemoryLimitMegabytes: 512, CpuLimitPercent: 50},
	})

	if out.Total != 2 {
		t.Fatalf("expected 2 servers, got %d", out.Total)
	}
	if out.Running != 1 {
		t.Fatalf("expected 1 running server, got %d", out.Running)
	}
	if out.MemoryBytes != 1024 {
		t.Fatalf("expected 1024 bytes of memory in use, got %d", out.MemoryBytes)
	}
	if out.CpuAbsolute != 12.5 {
		t.Fatalf("expected 12.5%% of cpu in use, got %f", out.CpuAbsolute)
	}
	if expected := int64(1536 * 1024 * 1024); out.MemoryAllocatedBytes != expected {
		t.Fatalf("expected %d bytes of memory allocated, got %d", expected, out.MemoryAllocatedBytes)
	}
	if out.CpuAllocatedPercent != 150 {
		t.Fatalf("expected 150%% of cpu allocated, got %d", out.CpuAllocatedPercent)
	}
}

func TestAggregateServersCountsUnlimitedSeparately(t *testing.T) {
	out := aggregateServers([]ServerUsage{
		{MemoryLimitMegabytes: 0, CpuLimitPercent: 0},
		{MemoryLimitMegabytes: 1024, CpuLimitPercent: 0},
		{MemoryLimitMegabytes: 1024, CpuLimitPercent: 100},
	})

	if out.Unlimited != 2 {
		t.Fatalf("expected 2 unlimited servers, got %d", out.Unlimited)
	}
	if expected := int64(2048 * 1024 * 1024); out.MemoryAllocatedBytes != expected {
		t.Fatalf("expected %d bytes of memory allocated, got %d", expected, out.MemoryAllocatedBytes)
	}
	if out.CpuAllocatedPercent != 100 {
		t.Fatalf("expected 100%% of cpu allocated, got %d", out.CpuAllocatedPercent)
	}
}

func TestAggregateServersWithoutServers(t *testing.T) {
	out := aggregateServers(nil)

	if out.Total != 0 || out.Running != 0 || out.Unlimited != 0 {
		t.Fatalf("expected an empty aggregate, got %+v", out)
	}
}
