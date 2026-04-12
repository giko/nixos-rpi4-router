package proc

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CPUTimes holds the aggregate CPU time counters from /proc/stat (in jiffies).
type CPUTimes struct {
	User   uint64
	Nice   uint64
	System uint64
	Idle   uint64
	IOWait uint64
}

// Total returns the sum of all CPU time fields.
func (c CPUTimes) Total() uint64 {
	return c.User + c.Nice + c.System + c.Idle + c.IOWait
}

// Delta returns the difference between the current and a previous CPUTimes
// snapshot. Useful for computing per-interval CPU percentages.
func (c CPUTimes) Delta(prev CPUTimes) CPUTimes {
	return CPUTimes{
		User:   c.User - prev.User,
		Nice:   c.Nice - prev.Nice,
		System: c.System - prev.System,
		Idle:   c.Idle - prev.Idle,
		IOWait: c.IOWait - prev.IOWait,
	}
}

// Stat holds parsed data from /proc/stat.
type Stat struct {
	CPU          CPUTimes
	BootTimeUnix uint64
}

// ReadStat parses the file at path (expected format: /proc/stat) and returns
// the aggregate CPU times and boot time. Only the "cpu " (aggregate, note
// trailing space) and "btime" lines are parsed; per-CPU lines are skipped.
func ReadStat(path string) (Stat, error) {
	f, err := os.Open(path)
	if err != nil {
		return Stat{}, fmt.Errorf("readstat: %w", err)
	}
	defer f.Close()

	var result Stat
	var foundCPU, foundBtime bool
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()

		// Aggregate CPU line starts with "cpu " (space after cpu).
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			if len(fields) < 6 {
				return Stat{}, fmt.Errorf("readstat: cpu line: expected at least 6 fields, got %d", len(fields))
			}

			vals := make([]uint64, 5)
			for i := 0; i < 5; i++ {
				v, err := strconv.ParseUint(fields[i+1], 10, 64)
				if err != nil {
					return Stat{}, fmt.Errorf("readstat: cpu field %d: %w", i, err)
				}
				vals[i] = v
			}

			result.CPU = CPUTimes{
				User:   vals[0],
				Nice:   vals[1],
				System: vals[2],
				Idle:   vals[3],
				IOWait: vals[4],
			}
			foundCPU = true
		}

		if strings.HasPrefix(line, "btime ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				return Stat{}, fmt.Errorf("readstat: btime line: expected 2 fields, got %d", len(fields))
			}
			v, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return Stat{}, fmt.Errorf("readstat: btime: %w", err)
			}
			result.BootTimeUnix = v
			foundBtime = true
		}

		if foundCPU && foundBtime {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return Stat{}, fmt.Errorf("readstat: %w", err)
	}

	if !foundCPU {
		return Stat{}, fmt.Errorf("readstat: no aggregate cpu line found")
	}
	if !foundBtime {
		return Stat{}, fmt.Errorf("readstat: no btime line found")
	}

	return result, nil
}
