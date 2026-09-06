package hoststats

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/disk"
	"golang.org/x/sys/unix"
)

// mount pairs a human readable label with the path it should be resolved from.
type mount struct {
	label string
	path  string
}

// filesystemID identifies the filesystem a path lives on so that directories
// sharing a filesystem can be reported as a single disk.
func filesystemID(path string) (string, bool) {
	var s unix.Statfs_t
	if err := unix.Statfs(path, &s); err != nil {
		return "", false
	}
	// The filesystem id alone is not unique on every kernel, so the device type
	// and total block count are folded in to make collisions unlikely.
	return fmt.Sprintf("%d:%d:%d:%d", s.Fsid.Val[0], s.Fsid.Val[1], s.Type, s.Blocks), true
}

// collectDisks resolves the usage of every given mount, merging the entries that
// turn out to live on the same filesystem. The order of the input is preserved,
// and the path reported for a merged entry is the one of its first label.
func collectDisks(mounts []mount) []Disk {
	out := make([]Disk, 0, len(mounts))
	// index maps a filesystem identifier onto its position in out.
	index := make(map[string]int, len(mounts))
	// seen tracks paths that could not be identified by filesystem so that the
	// same directory configured twice is still only reported once.
	seen := make(map[string]int, len(mounts))

	for _, m := range mounts {
		if m.path == "" {
			continue
		}

		key, ok := filesystemID(m.path)
		if !ok {
			key = ""
		}

		lookup := index
		if key == "" {
			key = m.path
			lookup = seen
		}

		if i, exists := lookup[key]; exists {
			out[i].Labels = append(out[i].Labels, m.label)
			continue
		}

		usage, err := disk.Usage(m.path)
		if err != nil {
			continue
		}

		lookup[key] = len(out)
		out = append(out, Disk{
			Labels:     []string{m.label},
			Path:       m.path,
			TotalBytes: usage.Total,
			UsedBytes:  usage.Used,
			Percent:    usage.UsedPercent,
		})
	}

	return out
}

// worstDiskPercent returns the utilization of the most saturated disk, which is
// what the disk pressure level is resolved from.
func worstDiskPercent(disks []Disk) float64 {
	var worst float64
	for _, d := range disks {
		if d.Percent > worst {
			worst = d.Percent
		}
	}
	return worst
}
