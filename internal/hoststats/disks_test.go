package hoststats

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectDisksMergesMountsOnTheSameFilesystem(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	disks := collectDisks([]mount{
		{label: "data", path: root},
		{label: "backups", path: nested},
	})

	if len(disks) != 1 {
		t.Fatalf("expected the two paths to be reported as a single disk, got %d", len(disks))
	}
	if got := disks[0].Labels; len(got) != 2 || got[0] != "data" || got[1] != "backups" {
		t.Fatalf("expected both labels on the merged disk, got %v", got)
	}
	if disks[0].Path != root {
		t.Fatalf("expected the path of the first label to be reported, got %q", disks[0].Path)
	}
}

func TestCollectDisksSkipsEmptyAndUnreadablePaths(t *testing.T) {
	disks := collectDisks([]mount{
		{label: "data", path: ""},
		{label: "backups", path: filepath.Join(t.TempDir(), "does-not-exist")},
	})

	if len(disks) != 0 {
		t.Fatalf("expected no disks to be reported, got %d", len(disks))
	}
}

func TestWorstDiskPercent(t *testing.T) {
	cases := []struct {
		name     string
		disks    []Disk
		expected float64
	}{
		{name: "no disks", disks: nil, expected: 0},
		{name: "single disk", disks: []Disk{{Percent: 42}}, expected: 42},
		{name: "worst of many", disks: []Disk{{Percent: 42}, {Percent: 91.5}, {Percent: 12}}, expected: 91.5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := worstDiskPercent(tc.disks); got != tc.expected {
				t.Fatalf("expected %f, got %f", tc.expected, got)
			}
		})
	}
}
