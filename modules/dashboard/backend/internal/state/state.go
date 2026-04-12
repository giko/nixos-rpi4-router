// Package state provides a thread-safe, in-memory cache of router state.
//
// Collectors write sections via Set*() methods; HTTP handlers read
// consistent snapshots via Snapshot*() methods. Each section carries
// an updated_at timestamp so handlers can detect stale data.
package state

import (
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

// State holds the cached router state. All fields are guarded by mu.
type State struct {
	mu sync.RWMutex

	system        model.SystemStats
	systemUpdated time.Time

	traffic        model.Traffic
	trafficUpdated time.Time

	tunnels        []model.Tunnel
	tunnelsUpdated time.Time

	pools        []model.Pool
	poolsUpdated time.Time

	clients        []model.Client
	clientsUpdated time.Time

	adguard        model.AdguardStats
	adguardUpdated time.Time

	clientConns        map[string]conntrack.ClientConnInfo
	clientConnsUpdated time.Time
}

// New returns an initialized State with zero values.
func New() *State {
	return &State{}
}

// --- System ---

// SetSystem replaces the cached system stats with a defensive copy.
func (s *State) SetSystem(v model.SystemStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.system = copySystem(v)
	s.systemUpdated = time.Now()
}

// SnapshotSystem returns a defensive copy of the cached system stats and its
// update time.
func (s *State) SnapshotSystem() (model.SystemStats, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySystem(s.system), s.systemUpdated
}

// --- Traffic ---

// SetTraffic replaces the cached traffic data. The interface slice is
// defensively copied so the caller can safely mutate the original afterward.
func (s *State) SetTraffic(v model.Traffic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traffic = model.Traffic{
		Interfaces: copyInterfaces(v.Interfaces),
	}
	s.trafficUpdated = time.Now()
}

// SnapshotTraffic returns a defensive copy of the cached traffic data and its
// update time.
func (s *State) SnapshotTraffic() (model.Traffic, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return model.Traffic{
		Interfaces: copyInterfaces(s.traffic.Interfaces),
	}, s.trafficUpdated
}

// --- Tunnels ---

// SetTunnels replaces the cached tunnel list with a defensive copy.
func (s *State) SetTunnels(v []model.Tunnel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunnels = copyTunnels(v)
	s.tunnelsUpdated = time.Now()
}

// SnapshotTunnels returns a defensive copy of the cached tunnels and the
// update time.
func (s *State) SnapshotTunnels() ([]model.Tunnel, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyTunnels(s.tunnels), s.tunnelsUpdated
}

// --- Pools ---

// SetPools replaces the cached pool list with a defensive copy.
func (s *State) SetPools(v []model.Pool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pools = copyPools(v)
	s.poolsUpdated = time.Now()
}

// SnapshotPools returns a defensive copy of the cached pools and the update
// time.
func (s *State) SnapshotPools() ([]model.Pool, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyPools(s.pools), s.poolsUpdated
}

// --- Clients ---

// SetClients replaces the cached client list with a defensive copy.
func (s *State) SetClients(v []model.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = copyClients(v)
	s.clientsUpdated = time.Now()
}

// SnapshotClients returns a defensive copy of the cached clients and the
// update time.
func (s *State) SnapshotClients() ([]model.Client, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyClients(s.clients), s.clientsUpdated
}

// SnapshotClient returns the client with the given IP, the section's update
// time, and a boolean indicating whether the client was found. The returned
// Client is a deep copy so callers can freely mutate it.
func (s *State) SnapshotClient(ip string) (model.Client, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.IP == ip {
			return copyClient(c), s.clientsUpdated, true
		}
	}
	return model.Client{}, s.clientsUpdated, false
}

// --- Adguard ---

// SetAdguard replaces the cached AdGuard stats with a defensive copy.
func (s *State) SetAdguard(v model.AdguardStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adguard = copyAdguard(v)
	s.adguardUpdated = time.Now()
}

// SnapshotAdguard returns a defensive copy of the cached AdGuard stats and
// the update time.
func (s *State) SnapshotAdguard() (model.AdguardStats, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyAdguard(s.adguard), s.adguardUpdated
}

// --- Client Fwmarks ---

// SetClientConns replaces the cached per-client connection info.
func (s *State) SetClientConns(m map[string]conntrack.ClientConnInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clientConns = make(map[string]conntrack.ClientConnInfo, len(m))
	for k, v := range m {
		// Deep-copy the TunnelConns map.
		cp := conntrack.ClientConnInfo{TotalConns: v.TotalConns}
		if v.TunnelConns != nil {
			cp.TunnelConns = make(map[string]int, len(v.TunnelConns))
			for mk, mv := range v.TunnelConns {
				cp.TunnelConns[mk] = mv
			}
		}
		s.clientConns[k] = cp
	}
	s.clientConnsUpdated = time.Now().UTC()
}

// SnapshotClientConns returns a defensive copy.
func (s *State) SnapshotClientConns() (map[string]conntrack.ClientConnInfo, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]conntrack.ClientConnInfo, len(s.clientConns))
	for k, v := range s.clientConns {
		cp := conntrack.ClientConnInfo{TotalConns: v.TotalConns}
		if v.TunnelConns != nil {
			cp.TunnelConns = make(map[string]int, len(v.TunnelConns))
			for mk, mv := range v.TunnelConns {
				cp.TunnelConns[mk] = mv
			}
		}
		out[k] = cp
	}
	return out, s.clientConnsUpdated
}

// --- Staleness ---

// IsStale reports whether updated is too old relative to the collection
// interval. A zero time is always stale. An update older than 2*interval
// is stale. The comparison is strict: exactly 2*interval is NOT stale.
func IsStale(updated time.Time, interval time.Duration) bool {
	if updated.IsZero() {
		return true
	}
	return time.Since(updated) > 2*interval
}

// --- defensive copy helpers ---

func copyInterfaces(src []model.Interface) []model.Interface {
	if src == nil {
		return nil
	}
	dst := make([]model.Interface, len(src))
	for i, iface := range src {
		dst[i] = iface
		if iface.Samples60s != nil {
			dst[i].Samples60s = make([]model.InterfaceSample, len(iface.Samples60s))
			copy(dst[i].Samples60s, iface.Samples60s)
		}
	}
	return dst
}

func copyTunnels(src []model.Tunnel) []model.Tunnel {
	if src == nil {
		return nil
	}
	dst := make([]model.Tunnel, len(src))
	copy(dst, src)
	return dst
}

func copyPools(src []model.Pool) []model.Pool {
	if src == nil {
		return nil
	}
	dst := make([]model.Pool, len(src))
	for i, p := range src {
		dst[i] = p
		dst[i].Members = make([]model.PoolMember, len(p.Members))
		copy(dst[i].Members, p.Members)
		dst[i].ClientIPs = make([]string, len(p.ClientIPs))
		copy(dst[i].ClientIPs, p.ClientIPs)
	}
	return dst
}

func copyClients(src []model.Client) []model.Client {
	if src == nil {
		return nil
	}
	dst := make([]model.Client, len(src))
	for i, c := range src {
		dst[i] = copyClient(c)
	}
	return dst
}

// copyClient deep-copies a Client so its map fields don't alias across
// the state cache and snapshot callers.
func copyClient(c model.Client) model.Client {
	out := c
	if c.TunnelConns != nil {
		out.TunnelConns = make(map[string]int, len(c.TunnelConns))
		for k, v := range c.TunnelConns {
			out.TunnelConns[k] = v
		}
	}
	return out
}

func copySystem(src model.SystemStats) model.SystemStats {
	dst := src
	if src.Services != nil {
		dst.Services = make([]model.ServiceState, len(src.Services))
		copy(dst.Services, src.Services)
	}
	return dst
}

func copyAdguard(src model.AdguardStats) model.AdguardStats {
	dst := src
	if src.TopBlocked != nil {
		dst.TopBlocked = make([]model.TopDomain, len(src.TopBlocked))
		copy(dst.TopBlocked, src.TopBlocked)
	}
	if src.TopClients != nil {
		dst.TopClients = make([]model.TopClient, len(src.TopClients))
		copy(dst.TopClients, src.TopClients)
	}
	if src.QueryDensity24h != nil {
		dst.QueryDensity24h = make([]model.DensityBin, len(src.QueryDensity24h))
		copy(dst.QueryDensity24h, src.QueryDensity24h)
	}
	return dst
}
