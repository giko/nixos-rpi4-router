package proc

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Meminfo holds parsed memory statistics from /proc/meminfo. All values are
// in bytes (the kernel reports in kB, we multiply by 1024).
type Meminfo struct {
	TotalBytes     uint64
	AvailableBytes uint64
	FreeBytes      uint64
}

// ReadMeminfo parses the file at path (expected format: /proc/meminfo) and
// returns total, available, and free memory in bytes.
func ReadMeminfo(path string) (Meminfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return Meminfo{}, fmt.Errorf("readmeminfo: %w", err)
	}
	defer f.Close()

	var result Meminfo
	found := 0
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		var target *uint64
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			target = &result.TotalBytes
		case strings.HasPrefix(line, "MemAvailable:"):
			target = &result.AvailableBytes
		case strings.HasPrefix(line, "MemFree:"):
			target = &result.FreeBytes
		default:
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			return Meminfo{}, fmt.Errorf("readmeminfo: malformed line: %s", line)
		}

		kB, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return Meminfo{}, fmt.Errorf("readmeminfo: %s: %w", fields[0], err)
		}

		*target = kB * 1024
		found++

		if found == 3 {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return Meminfo{}, fmt.Errorf("readmeminfo: %w", err)
	}

	if found < 3 {
		return Meminfo{}, fmt.Errorf("readmeminfo: incomplete data, found %d of 3 required fields", found)
	}

	return result, nil
}
