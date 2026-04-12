package wireguard

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Dump holds parsed output from `wg show <iface> dump`.
type Dump struct {
	LocalPrivateKey string
	LocalPublicKey  string
	ListenPort      int
	Fwmark          string
	Peers           []Peer
}

// Peer holds a single peer row from the dump output.
type Peer struct {
	PublicKey            string
	PresharedKey         string
	Endpoint             string
	AllowedIPs           string
	LatestHandshakeUnix  int64
	RXBytes              uint64
	TXBytes              uint64
	PersistentKeepalive  int
}

// ParseDump parses the tab-separated output of `wg show <iface> dump`.
//
// The first line is the interface header with 4 tab-separated fields:
//
//	private-key  public-key  listen-port  fwmark
//
// Subsequent lines are peer rows with 8 tab-separated fields:
//
//	public-key  preshared-key  endpoint  allowed-ips  latest-handshake  transfer-rx  transfer-tx  persistent-keepalive
func ParseDump(raw string) (Dump, error) {
	raw = strings.TrimRight(raw, "\n")
	if raw == "" {
		return Dump{}, fmt.Errorf("parsedump: empty input")
	}

	lines := strings.Split(raw, "\n")

	// Parse interface header (first line).
	hdr := strings.Split(lines[0], "\t")
	if len(hdr) < 4 {
		return Dump{}, fmt.Errorf("parsedump: interface line: expected 4 fields, got %d", len(hdr))
	}

	port, err := strconv.Atoi(hdr[2])
	if err != nil {
		return Dump{}, fmt.Errorf("parsedump: listen-port: %w", err)
	}

	d := Dump{
		LocalPrivateKey: hdr[0],
		LocalPublicKey:  hdr[1],
		ListenPort:      port,
		Fwmark:          hdr[3],
	}

	// Parse peer rows.
	for i, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			return Dump{}, fmt.Errorf("parsedump: peer line %d: expected 8 fields, got %d", i+2, len(fields))
		}

		handshake, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return Dump{}, fmt.Errorf("parsedump: peer line %d: latest-handshake: %w", i+2, err)
		}

		rx, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return Dump{}, fmt.Errorf("parsedump: peer line %d: rx-bytes: %w", i+2, err)
		}

		tx, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return Dump{}, fmt.Errorf("parsedump: peer line %d: tx-bytes: %w", i+2, err)
		}

		keepalive, err := strconv.Atoi(fields[7])
		if err != nil {
			return Dump{}, fmt.Errorf("parsedump: peer line %d: persistent-keepalive: %w", i+2, err)
		}

		d.Peers = append(d.Peers, Peer{
			PublicKey:           fields[0],
			PresharedKey:        fields[1],
			Endpoint:            fields[2],
			AllowedIPs:          fields[3],
			LatestHandshakeUnix: handshake,
			RXBytes:             rx,
			TXBytes:             tx,
			PersistentKeepalive: keepalive,
		})
	}

	return d, nil
}

// Show executes `wg show <iface> dump` and parses the output.
func Show(ctx context.Context, iface string) (Dump, error) {
	cmd := exec.CommandContext(ctx, "wg", "show", iface, "dump")
	out, err := cmd.Output()
	if err != nil {
		return Dump{}, fmt.Errorf("wg show %s dump: %w", iface, err)
	}
	return ParseDump(string(out))
}
