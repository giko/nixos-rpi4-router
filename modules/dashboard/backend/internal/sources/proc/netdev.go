package proc

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// NetDevStats holds parsed counters for a single network interface from
// /proc/net/dev.
type NetDevStats struct {
	Interface string
	RXBytes   uint64
	RXPackets uint64
	TXBytes   uint64
	TXPackets uint64
}

// ReadNetDev parses the file at path (expected format: /proc/net/dev) and
// returns per-interface statistics keyed by interface name.
//
// The first two lines are headers and are skipped. Each subsequent line has
// the form:
//
//	iface: rx_bytes rx_packets rx_errs rx_drop rx_fifo rx_frame rx_compressed rx_multicast tx_bytes tx_packets tx_errs tx_drop tx_fifo tx_colls tx_carrier tx_compressed
func ReadNetDev(path string) (map[string]NetDevStats, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("readnetdev: %w", err)
	}
	defer f.Close()

	result := make(map[string]NetDevStats)
	scanner := bufio.NewScanner(f)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		if lineNo <= 2 {
			continue // skip header lines
		}

		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return nil, fmt.Errorf("readnetdev: line %d: expected 16 fields, got %d", lineNo, len(fields))
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("readnetdev: line %d: rx_bytes: %w", lineNo, err)
		}
		rxPackets, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("readnetdev: line %d: rx_packets: %w", lineNo, err)
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("readnetdev: line %d: tx_bytes: %w", lineNo, err)
		}
		txPackets, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("readnetdev: line %d: tx_packets: %w", lineNo, err)
		}

		result[iface] = NetDevStats{
			Interface: iface,
			RXBytes:   rxBytes,
			RXPackets: rxPackets,
			TXBytes:   txBytes,
			TXPackets: txPackets,
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("readnetdev: %w", err)
	}

	return result, nil
}
