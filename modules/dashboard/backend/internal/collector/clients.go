package collector

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/dnsmasq"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// NeighFunc returns a map of IP → MAC from the neighbour table.
// Wraps ipneigh.Collect so tests can inject a fake.
type NeighFunc func(ctx context.Context) (map[string]string, error)

// ClientsOpts configures the Clients collector.
type ClientsOpts struct {
	Topology   *topology.Topology
	LeasesPath string
	State      *state.State
	Neigh      NeighFunc // nil is valid (skips neighbour discovery)
}

// Clients is a medium-tier collector that merges static leases, dynamic
// leases, ip-neigh, and client fwmarks into []model.Client.
type Clients struct {
	opts         ClientsOpts
	tunnelByMark map[string]string // fwmark hex → tunnel name
	poolByIP     map[string]string // IP → pool name
	allowed      map[string]struct{}
}

// NewClients creates a Clients collector. Precomputes lookup tables from
// topology so Run only does per-tick work.
func NewClients(opts ClientsOpts) *Clients {
	// Build fwmark → tunnel name map.
	tunnelByMark := make(map[string]string, len(opts.Topology.Tunnels))
	for _, t := range opts.Topology.Tunnels {
		if t.Fwmark != "" {
			tunnelByMark[t.Fwmark] = t.Name
		}
	}

	// Build IP → pool name map from pooled rules.
	poolByIP := make(map[string]string)
	for _, r := range opts.Topology.PooledRules {
		for _, ip := range r.Sources {
			poolByIP[ip] = r.Pool
		}
	}

	// Build allowed MAC set: explicit AllowedMACs + all static lease MACs.
	allowed := make(map[string]struct{})
	for _, mac := range opts.Topology.AllowedMACs {
		allowed[strings.ToLower(mac)] = struct{}{}
	}
	for _, sl := range opts.Topology.StaticLeases {
		allowed[strings.ToLower(sl.MAC)] = struct{}{}
	}

	return &Clients{
		opts:         opts,
		tunnelByMark: tunnelByMark,
		poolByIP:     poolByIP,
		allowed:      allowed,
	}
}

func (*Clients) Name() string { return "clients" }
func (*Clients) Tier() Tier   { return Medium }

// Run performs a single collection pass.
func (c *Clients) Run(ctx context.Context) error {
	// Indexed set of IPs already seen, used to avoid duplicates.
	seen := make(map[string]struct{})
	var clients []model.Client

	// 1. Static leases from topology.
	for _, sl := range c.opts.Topology.StaticLeases {
		seen[sl.IP] = struct{}{}
		clients = append(clients, model.Client{
			Hostname:  sl.Name,
			IP:        sl.IP,
			MAC:       strings.ToLower(sl.MAC),
			LeaseType: "static",
		})
	}

	// 2. Dynamic leases from dnsmasq.
	leases, err := dnsmasq.ReadLeases(c.opts.LeasesPath)
	if err != nil {
		return err
	}
	for _, l := range leases {
		if _, ok := seen[l.IP]; ok {
			continue
		}
		seen[l.IP] = struct{}{}
		cl := model.Client{
			Hostname:  l.Hostname,
			IP:        l.IP,
			MAC:       strings.ToLower(l.MAC),
			LeaseType: "dynamic",
		}
		if l.ExpireUnix > 0 {
			cl.LastSeen = l.ExpiresAt()
		}
		clients = append(clients, cl)
	}

	// 3. Neighbour table.
	if c.opts.Neigh != nil {
		neigh, err := c.opts.Neigh(ctx)
		if err == nil {
			for ip, mac := range neigh {
				if _, ok := seen[ip]; ok {
					continue
				}
				seen[ip] = struct{}{}
				clients = append(clients, model.Client{
					IP:        ip,
					MAC:       strings.ToLower(mac),
					LeaseType: "neighbor",
				})
			}
		}
		// Neighbour failure is non-fatal; we already have leases.
	}

	// 4. Enrich with connection info and route derivation.
	connInfo, _ := c.opts.State.SnapshotClientConns()

	// 4a. Synthesise clients for conntrack-only IPs that aren't in any
	// lease or neighbour record. Without this, a static-IP device with
	// an aged-out ARP entry (common on server subnets) would silently
	// disappear from the clients list — and, more importantly, from
	// pool connection totals that aggregate client.tunnel_conns.
	for ip := range connInfo {
		if _, ok := seen[ip]; ok {
			continue
		}
		seen[ip] = struct{}{}
		clients = append(clients, model.Client{
			IP:        ip,
			LeaseType: "conntrack",
		})
	}

	now := time.Now()
	for i := range clients {
		cl := &clients[i]

		// Connection count + per-tunnel breakdown from conntrack cold tier.
		if info, ok := connInfo[cl.IP]; ok {
			cl.ConnCount = info.TotalConns
			if len(info.TunnelConns) > 0 {
				cl.TunnelConns = make(map[string]int, len(info.TunnelConns))
				for mark, count := range info.TunnelConns {
					cl.TunnelConns[mark] = count
				}
			}
		}

		// Route derivation — based on pool membership (from topology),
		// not on individual connection marks. In a round-robin pool,
		// a client's connections are spread across ALL tunnels, so
		// there is no single "current tunnel".
		if pool, ok := c.poolByIP[cl.IP]; ok {
			cl.Route = "pool:" + pool
		} else {
			// Non-pooled client: pick the configured tunnel with the
			// most connections. We only consider marks that resolve to
			// a known tunnel (ignoring e.g. the 0x10000 WAN-forced mark
			// used by the router) and tie-break deterministically by
			// tunnel name so a split of equal connections doesn't flap
			// the displayed route between refreshes.
			cl.Route = "wan"
			if info, ok := connInfo[cl.IP]; ok {
				type tunStat struct {
					name  string
					count int
				}
				var stats []tunStat
				for mark, count := range info.TunnelConns {
					if tun, ok := c.tunnelByMark[mark]; ok {
						stats = append(stats, tunStat{name: tun, count: count})
					}
				}
				if len(stats) > 0 {
					sort.Slice(stats, func(i, j int) bool {
						if stats[i].count != stats[j].count {
							return stats[i].count > stats[j].count
						}
						return stats[i].name < stats[j].name
					})
					cl.Route = "tunnel:" + stats[0].name
				}
			}
		}

		// Allowlist status.
		if !c.opts.Topology.AllowlistEnabled {
			cl.AllowlistStatus = "n/a"
		} else if cl.MAC == "" {
			cl.AllowlistStatus = "n/a"
		} else if _, ok := c.allowed[cl.MAC]; ok {
			cl.AllowlistStatus = "allowed"
		} else {
			cl.AllowlistStatus = "blocked"
		}

		// Default LastSeen to now for non-dynamic (static/neighbor).
		if cl.LastSeen.IsZero() {
			cl.LastSeen = now
		}
	}

	// Sort by IP for stable output.
	sort.Slice(clients, func(i, j int) bool {
		return compareIPs(clients[i].IP, clients[j].IP) < 0
	})

	c.opts.State.SetClients(clients)
	return nil
}

// compareIPs compares two IPv4 address strings numerically.
// Falls back to lexicographic comparison for non-IPv4.
func compareIPs(a, b string) int {
	pa := net.ParseIP(a)
	pb := net.ParseIP(b)
	if pa == nil || pb == nil {
		return strings.Compare(a, b)
	}
	pa = pa.To4()
	pb = pb.To4()
	if pa == nil || pb == nil {
		return strings.Compare(a, b)
	}
	for i := 0; i < 4; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}
