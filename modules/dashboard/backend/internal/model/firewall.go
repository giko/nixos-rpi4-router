package model

// Firewall is the snapshot served by three endpoints:
//
//	/api/firewall/rules     → {port_forwards, pbr, allowed_macs, blocked_forward_count_1h}
//	/api/firewall/counters  → {chains: [...]}
//	/api/upnp               → {leases: [...]}
//
// A single snapshot holds all three projections because one nft parse
// and one topology load produce everything at once; the handlers carve
// out their own fields.
type Firewall struct {
	// Rules — static config exposed verbatim from topology.
	PortForwards []PortForward `json:"port_forwards"`
	PBR          PBR           `json:"pbr"`
	AllowedMACs  []string      `json:"allowed_macs"`

	// BlockedForwardCount1h is the delta of summed packet counters on
	// the forward chain's drop rules over the trailing ~60 minutes.
	// Rolled forward by the collector's in-memory ring buffer.
	BlockedForwardCount1h int64 `json:"blocked_forward_count_1h"`

	// Chains carries the dynamic counter view: every nft chain with
	// its per-rule counters nested. Served by /api/firewall/counters.
	Chains []FirewallChain `json:"chains"`

	// UPnPLeases are active port mappings derived from the inet/miniupnpd
	// nft table (miniupnpd has no on-disk lease file in this config).
	UPnPLeases []UPnPLease `json:"upnp_leases"`
}

// PortForward mirrors router.portForwards[].
type PortForward struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	Destination  string `json:"destination"`
}

// PBR bundles the three PBR rule kinds the dashboard surfaces.
// `pooled_rules` reuses Pool topology so the frontend can render
// which clients feed which pool.
type PBR struct {
	SourceRules []PBRSourceRule `json:"source_rules"`
	DomainRules []PBRDomainRule `json:"domain_rules"`
	PooledRules []PBRPooledRule `json:"pooled_rules"`
}

type PBRSourceRule struct {
	Sources []string `json:"sources"`
	Tunnel  string   `json:"tunnel"`
}

type PBRDomainRule struct {
	Tunnel  string   `json:"tunnel"`
	Domains []string `json:"domains"`
}

type PBRPooledRule struct {
	Sources []string `json:"sources"`
	Pool    string   `json:"pool"`
}

// FirewallChain carries its per-rule counters nested — the /counters
// endpoint serves `{chains: [...]}` exactly like this.
type FirewallChain struct {
	Family   string        `json:"family"`
	Table    string        `json:"table"`
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Hook     string        `json:"hook"`
	Priority int           `json:"priority"`
	Policy   string        `json:"policy"`
	Handle   int           `json:"handle"`
	Counters []RuleCounter `json:"counters"`
}

// RuleCounter pairs one nft counter expression with the rule handle
// that owns it and the comment on that rule.
type RuleCounter struct {
	Handle  int    `json:"handle"`
	Comment string `json:"comment,omitempty"`
	Packets int64  `json:"packets"`
	Bytes   int64  `json:"bytes"`
}

// UPnPLease is one active port mapping established by miniupnpd.
// Named "lease" in the API surface (§7.4) even though miniupnpd
// can't attach an explicit TTL in this config — the nft rule's
// existence is the lease.
type UPnPLease struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	InternalAddr string `json:"internal_addr"`
	InternalPort int    `json:"internal_port"`
	Description  string `json:"description,omitempty"`
}
