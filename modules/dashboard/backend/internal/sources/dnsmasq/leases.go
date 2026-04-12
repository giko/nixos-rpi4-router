package dnsmasq

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

// Lease is one row from dnsmasq.leases.
type Lease struct {
	ExpireUnix int64
	MAC        string
	IP         string
	Hostname   string // "" when dnsmasq logs "*"
	ClientID   string // "" when dnsmasq logs "*"
}

// ExpiresAt returns the expiry as a time.Time.
func (l Lease) ExpiresAt() time.Time {
	return time.Unix(l.ExpireUnix, 0)
}

// ReadLeases parses the dnsmasq.leases file at path.
// Format: <expire_unix> <mac> <ip> <hostname_or_star> <client_id_or_star>
// Missing file returns nil, nil (no leases, no error).
func ReadLeases(path string) ([]Lease, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("readleases: %w", err)
	}

	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return nil, nil
	}

	var leases []Lease
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		expire, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}

		hostname := fields[3]
		if hostname == "*" {
			hostname = ""
		}

		clientID := fields[4]
		if clientID == "*" {
			clientID = ""
		}

		leases = append(leases, Lease{
			ExpireUnix: expire,
			MAC:        strings.ToLower(fields[1]),
			IP:         fields[2],
			Hostname:   hostname,
			ClientID:   clientID,
		})
	}
	return leases, nil
}
