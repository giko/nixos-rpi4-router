package proc

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ReadThermal reads a thermal zone temperature file (e.g.
// /sys/class/thermal/thermal_zone0/temp). The file contains a single integer
// representing millidegrees Celsius (e.g. "52314" = 52.314 C).
func ReadThermal(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("readthermal: %w", err)
	}

	millideg, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("readthermal: %w", err)
	}

	return float64(millideg) / 1000.0, nil
}

// ReadUptime reads /proc/uptime and returns the system uptime in seconds.
// The file contains two space-separated floats; only the first (uptime) is
// returned.
func ReadUptime(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("readuptime: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) < 1 {
		return 0, fmt.Errorf("readuptime: empty file")
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("readuptime: %w", err)
	}

	return uptime, nil
}
