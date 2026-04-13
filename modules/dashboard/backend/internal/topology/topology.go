// Package topology loads the NixOS-generated dashboard configuration
// describing tunnels, pools, static leases, and allowlist state.
package topology

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Topology holds the router topology exported by the NixOS module.
type Topology struct {
	Tunnels          []Tunnel        `json:"tunnels"`
	Pools            []Pool          `json:"pools"`
	PooledRules      []PooledRule    `json:"pooled_rules"`
	StaticLeases     []StaticLease   `json:"static_leases"`
	AllowlistEnabled bool            `json:"allowlist_enabled"`
	AllowedMACs      []string        `json:"allowed_macs"`
	PortForwards     []PortForward   `json:"port_forwards"`
	PBRSourceRules   []PBRSourceRule `json:"pbr_source_rules"`
	PBRDomainRules   []PBRDomainRule `json:"pbr_domain_rules"`
	LANInterface     string          `json:"lan_interface"`
	WANInterface     string          `json:"wan_interface"`
}

// Tunnel describes a WireGuard tunnel with its fwmark and routing table.
type Tunnel struct {
	Name         string `json:"name"`
	Interface    string `json:"interface"`
	Fwmark       string `json:"fwmark"`
	RoutingTable int    `json:"routing_table"`
}

// Pool groups tunnels for round-robin or failover assignment.
type Pool struct {
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// PooledRule maps source IPs to a named pool.
type PooledRule struct {
	Sources []string `json:"sources"`
	Pool    string   `json:"pool"`
}

// StaticLease is a DHCP static lease entry.
type StaticLease struct {
	MAC  string `json:"mac"`
	IP   string `json:"ip"`
	Name string `json:"name,omitempty"`
}

// Load reads and parses a topology JSON file. An empty path or missing
// file returns a zero-value Topology (dev mode). Malformed JSON is a
// hard error (config bug).
func Load(path string) (*Topology, error) {
	if path == "" {
		return &Topology{}, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Topology{}, nil
		}
		return nil, fmt.Errorf("read topology %s: %w", path, err)
	}
	var t Topology
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("parse topology %s: %w", path, err)
	}
	return &t, nil
}

// TunnelByName returns a pointer to the tunnel with the given name,
// or nil if not found.
func (t *Topology) TunnelByName(name string) *Tunnel {
	for i := range t.Tunnels {
		if t.Tunnels[i].Name == name {
			return &t.Tunnels[i]
		}
	}
	return nil
}

// PoolByName returns a pointer to the pool with the given name,
// or nil if not found.
func (t *Topology) PoolByName(name string) *Pool {
	for i := range t.Pools {
		if t.Pools[i].Name == name {
			return &t.Pools[i]
		}
	}
	return nil
}

// ClientIPsForPool returns all source IPs from pooled rules that
// reference the given pool name, deduplicated.
func (t *Topology) ClientIPsForPool(name string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, r := range t.PooledRules {
		if r.Pool != name {
			continue
		}
		for _, ip := range r.Sources {
			if _, ok := seen[ip]; ok {
				continue
			}
			seen[ip] = struct{}{}
			out = append(out, ip)
		}
	}
	return out
}

// PortForward is a static DNAT rule defined in nix module config.
type PortForward struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	Destination  string `json:"destination"` // "ip:port"
}

// PBRSourceRule routes every new connection from `Sources` through `Tunnel`.
type PBRSourceRule struct {
	Sources []string `json:"sources"`
	Tunnel  string   `json:"tunnel"` // tunnel name or "wan"
}

// PBRDomainRule routes traffic to `Domains` through `Tunnel`.
type PBRDomainRule struct {
	Tunnel  string   `json:"tunnel"`
	Domains []string `json:"domains"`
}
