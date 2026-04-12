// Package state provides a thread-safe, in-memory cache of router state.
//
// Collectors write sections via Set*() methods; HTTP handlers read
// consistent snapshots via Snapshot*() methods. Each section carries
// an updated_at timestamp so handlers can detect stale data.
package state

import (
	"sync"
	"time"

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
}

// New returns an initialized State with zero values.
func New() *State {
	return &State{}
}

// --- System ---

// SetSystem replaces the cached system stats.
func (s *State) SetSystem(v model.SystemStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.system = v
	s.systemUpdated = time.Now()
}

// SnapshotSystem returns a copy of the cached system stats and its update time.
func (s *State) SnapshotSystem() (model.SystemStats, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.system, s.systemUpdated
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
// time, and a boolean indicating whether the client was found.
func (s *State) SnapshotClient(ip string) (model.Client, time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.IP == ip {
			return c, s.clientsUpdated, true
		}
	}
	return model.Client{}, s.clientsUpdated, false
}

// --- Adguard ---

// SetAdguard replaces the cached AdGuard stats.
func (s *State) SetAdguard(v model.AdguardStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adguard = v
	s.adguardUpdated = time.Now()
}

// SnapshotAdguard returns a copy of the cached AdGuard stats and the update
// time.
func (s *State) SnapshotAdguard() (model.AdguardStats, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.adguard, s.adguardUpdated
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
	copy(dst, src)
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
	copy(dst, src)
	return dst
}
