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

	// system holds the merged view across hot (CPU/memory/temp/uptime) and
	// medium (services/throttled) tiers. Each tier tracks its own
	// updated_at so a stale hot pass can't mask a fresh medium pass and
	// vice versa.
	system              model.SystemStats
	systemHotUpdated    time.Time
	systemMediumUpdated time.Time

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

	firewall        model.Firewall
	firewallUpdated time.Time

	qos        model.QoS
	qosUpdated time.Time
}

// New returns an initialized State with zero values.
func New() *State {
	return &State{}
}

// --- System ---

// SetSystem replaces the cached system stats with a defensive copy and
// stamps both hot and medium updated_at. Retained for tests and legacy
// callers that still want a one-shot seed; production collectors should
// use SetSystemHot / SetSystemMedium so each tier's freshness is tracked
// independently.
func (s *State) SetSystem(v model.SystemStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.system = copySystem(v)
	now := time.Now()
	s.systemHotUpdated = now
	s.systemMediumUpdated = now
}

// SetSystemHot updates only hot-tier system fields (CPU / memory / temp /
// uptime / boot time). Medium-tier fields (Services, Throttled,
// ThrottledFlag) are preserved under the state mutex. This avoids the
// snapshot-modify-replace race where a slower hot pass could clobber
// fresh medium data.
func (s *State) SetSystemHot(cpu model.CPUStats, mem model.MemoryStats, temp float64, uptime float64, boot time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.system.CPU = cpu
	s.system.Memory = mem
	s.system.TemperatureC = temp
	s.system.UptimeSeconds = uptime
	s.system.BootTime = boot
	s.systemHotUpdated = time.Now()
}

// SetSystemMedium updates only medium-tier system fields (Services,
// Throttled, ThrottledFlag). Hot-tier fields are preserved.
func (s *State) SetSystemMedium(services []model.ServiceState, throttled string, throttledFlag bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if services != nil {
		cp := make([]model.ServiceState, len(services))
		copy(cp, services)
		s.system.Services = cp
	} else {
		s.system.Services = nil
	}
	s.system.Throttled = throttled
	s.system.ThrottledFlag = throttledFlag
	s.systemMediumUpdated = time.Now()
}

// SnapshotSystem returns a defensive copy of the merged system stats and
// the oldest of the hot/medium update timestamps. Using the oldest
// ensures that a stale tier still surfaces through the handler's
// freshness check instead of being masked by a fresher sibling.
func (s *State) SnapshotSystem() (model.SystemStats, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySystem(s.system), oldest(s.systemHotUpdated, s.systemMediumUpdated)
}

// SnapshotSystemTiers returns a copy of the cached system stats together
// with the hot-tier and medium-tier updated_at timestamps. Handlers can
// use whichever timestamp is relevant for the field they serve, or the
// oldest (via SnapshotSystem) for the merged view.
func (s *State) SnapshotSystemTiers() (model.SystemStats, time.Time, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copySystem(s.system), s.systemHotUpdated, s.systemMediumUpdated
}

// oldest returns the earlier of two times, treating zero as "newest" so
// a never-populated tier doesn't drag the merged timestamp to zero.
// When both are zero the result is zero (genuinely never populated).
func oldest(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() {
		return a
	}
	if a.Before(b) {
		return a
	}
	return b
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

// SetPools replaces the cached pool list with a defensive copy. Retained
// for tests and seed writes; production collectors should use
// SetPoolsHot / SetPoolFlows so hot topology updates don't clobber fresh
// cold-tier flow counts and vice versa.
func (s *State) SetPools(v []model.Pool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pools = copyPools(v)
	s.poolsUpdated = time.Now()
}

// SetPoolsHot writes only topology-derived pool fields (Name, Members'
// Tunnel/Fwmark/Healthy, ClientIPs, FailsafeDropActive) and preserves the
// cold-tier FlowCount already held in state. Incoming members without a
// match in the previous state get FlowCount=0.
func (s *State) SetPoolsHot(v []model.Pool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Index existing flow counts by pool name → tunnel name so we can
	// preserve them across the in-place merge.
	prevFlows := make(map[string]map[string]int, len(s.pools))
	for _, p := range s.pools {
		m := make(map[string]int, len(p.Members))
		for _, mem := range p.Members {
			m[mem.Tunnel] = mem.FlowCount
		}
		prevFlows[p.Name] = m
	}

	merged := copyPools(v)
	for i := range merged {
		if prev, ok := prevFlows[merged[i].Name]; ok {
			for j := range merged[i].Members {
				if fc, ok := prev[merged[i].Members[j].Tunnel]; ok {
					merged[i].Members[j].FlowCount = fc
				}
			}
		}
	}
	s.pools = merged
	s.poolsUpdated = time.Now()
}

// SetPoolFlows updates only the per-member FlowCount values. counts is
// keyed by pool name → tunnel name → flow count. Any pool or member not
// present in counts keeps its previous FlowCount; any count referring to
// an unknown pool/member is silently ignored (the hot-tier collector is
// the source of truth for pool membership).
func (s *State) SetPoolFlows(counts map[string]map[string]int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.pools {
		per, ok := counts[s.pools[i].Name]
		if !ok {
			continue
		}
		for j := range s.pools[i].Members {
			if fc, ok := per[s.pools[i].Members[j].Tunnel]; ok {
				s.pools[i].Members[j].FlowCount = fc
			}
		}
	}
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

// --- Firewall ---

// SetFirewall replaces the cached firewall snapshot with a defensive copy.
func (s *State) SetFirewall(v model.Firewall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firewall = copyFirewall(v)
	s.firewallUpdated = time.Now()
}

// SnapshotFirewall returns a defensive copy of the firewall snapshot
// and the section's update time.
func (s *State) SnapshotFirewall() (model.Firewall, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyFirewall(s.firewall), s.firewallUpdated
}

// --- QoS ---

// SetQoS replaces the cached QoS snapshot with a defensive copy.
func (s *State) SetQoS(v model.QoS) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qos = copyQoS(v)
	s.qosUpdated = time.Now()
}

// SnapshotQoS returns a defensive copy of the QoS snapshot and the
// section's update time.
func (s *State) SnapshotQoS() (model.QoS, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyQoS(s.qos), s.qosUpdated
}

func copyFirewall(src model.Firewall) model.Firewall {
	dst := model.Firewall{
		BlockedForwardCount1h: src.BlockedForwardCount1h,
	}
	if src.PortForwards != nil {
		dst.PortForwards = make([]model.PortForward, len(src.PortForwards))
		copy(dst.PortForwards, src.PortForwards)
	}
	dst.PBR = copyPBR(src.PBR)
	if src.AllowedMACs != nil {
		dst.AllowedMACs = make([]string, len(src.AllowedMACs))
		copy(dst.AllowedMACs, src.AllowedMACs)
	}
	if src.Chains != nil {
		dst.Chains = make([]model.FirewallChain, len(src.Chains))
		for i, c := range src.Chains {
			dst.Chains[i] = c
			if c.Counters != nil {
				dst.Chains[i].Counters = make([]model.RuleCounter, len(c.Counters))
				copy(dst.Chains[i].Counters, c.Counters)
			}
		}
	}
	if src.UPnPLeases != nil {
		dst.UPnPLeases = make([]model.UPnPLease, len(src.UPnPLeases))
		copy(dst.UPnPLeases, src.UPnPLeases)
	}
	return dst
}

func copyPBR(src model.PBR) model.PBR {
	dst := model.PBR{}
	if src.SourceRules != nil {
		dst.SourceRules = make([]model.PBRSourceRule, len(src.SourceRules))
		for i, r := range src.SourceRules {
			dst.SourceRules[i] = r
			if r.Sources != nil {
				dst.SourceRules[i].Sources = append([]string(nil), r.Sources...)
			}
		}
	}
	if src.DomainRules != nil {
		dst.DomainRules = make([]model.PBRDomainRule, len(src.DomainRules))
		for i, r := range src.DomainRules {
			dst.DomainRules[i] = r
			if r.Domains != nil {
				dst.DomainRules[i].Domains = append([]string(nil), r.Domains...)
			}
		}
	}
	if src.PooledRules != nil {
		dst.PooledRules = make([]model.PBRPooledRule, len(src.PooledRules))
		for i, r := range src.PooledRules {
			dst.PooledRules[i] = r
			if r.Sources != nil {
				dst.PooledRules[i].Sources = append([]string(nil), r.Sources...)
			}
		}
	}
	return dst
}

func copyQoS(src model.QoS) model.QoS {
	dst := model.QoS{}
	if src.Egress != nil {
		eg := *src.Egress
		if src.Egress.Tins != nil {
			eg.Tins = make([]model.CAKETin, len(src.Egress.Tins))
			copy(eg.Tins, src.Egress.Tins)
		}
		dst.Egress = &eg
	}
	if src.Ingress != nil {
		in := *src.Ingress
		if src.Ingress.Tins != nil {
			in.Tins = make([]model.CAKETin, len(src.Ingress.Tins))
			copy(in.Tins, src.Ingress.Tins)
		}
		dst.Ingress = &in
	}
	return dst
}
