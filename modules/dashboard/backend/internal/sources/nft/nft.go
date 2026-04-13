// Package nft parses `nft --json list ruleset` output into a typed
// shape the dashboard's firewall/UPnP collectors consume. The on-wire
// JSON is a flat array under "nftables[]" of heterogeneous entries
// (table / chain / rule / set / counter); we walk it once, building
// the by-chain index and extracting per-rule counters and UPnP DNAT
// mappings in the same pass.
package nft

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Ruleset is the parsed projection of `nft --json list ruleset`.
type Ruleset struct {
	Chains       []Chain       // every chain across every table
	Counters     []Counter     // every {"counter": {...}} expression encountered, tied back to chain+rule handle
	UPnPMappings []UPnPMapping // DNAT rules extracted from inet/miniupnpd chains
}

// Chain summarises one nft chain.
type Chain struct {
	Family    string `json:"family"` // "ip" | "ip6" | "inet" | ...
	Table     string `json:"table"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"` // "filter" | "nat" | ""
	Hook      string `json:"hook,omitempty"` // "input" | "forward" | "prerouting" | ...
	Priority  int    `json:"priority,omitempty"`
	Policy    string `json:"policy,omitempty"` // "accept" | "drop" | ""
	Handle    int    `json:"handle"`
	RuleCount int    `json:"rule_count"`
}

// Counter is one inline counter expression's value, tagged with the
// chain it lives in, the rule's nft handle, and the rule's
// terminating verdict (drop / accept / return / jump / "" when the
// rule has none — e.g. a mangle rule).
type Counter struct {
	Family    string `json:"family"`
	Table     string `json:"table"`
	ChainName string `json:"chain"`
	Handle    int    `json:"handle"`
	Comment   string `json:"comment,omitempty"`
	Verdict   string `json:"verdict,omitempty"`
	Packets   int64  `json:"packets"`
	Bytes     int64  `json:"bytes"`
}

// UPnPMapping is one active port forward established by miniupnpd.
type UPnPMapping struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	InternalAddr string `json:"internal_addr"`
	InternalPort int    `json:"internal_port"`
	Description  string `json:"description,omitempty"`
}

// Runner executes nft and returns its stdout. Tests inject a fake.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultRunner runs the real nft binary with the given args.
func DefaultRunner(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "nft", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("nft %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Collect runs `nft --json list ruleset` via the given runner and
// returns a parsed Ruleset.
func Collect(ctx context.Context, run Runner) (*Ruleset, error) {
	raw, err := run(ctx, "--json", "list", "ruleset")
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse decodes the raw nft JSON envelope and projects it into Ruleset.
func Parse(raw []byte) (*Ruleset, error) {
	var env struct {
		Nftables []json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("nft: parse envelope: %w", err)
	}

	out := &Ruleset{
		Chains:       []Chain{},
		Counters:     []Counter{},
		UPnPMappings: []UPnPMapping{},
	}
	chainsByKey := make(map[string]int) // family/table/chain → index in out.Chains

	for _, entry := range env.Nftables {
		var top map[string]json.RawMessage
		if err := json.Unmarshal(entry, &top); err != nil {
			continue
		}
		if raw, ok := top["chain"]; ok {
			var c struct {
				Family string `json:"family"`
				Table  string `json:"table"`
				Name   string `json:"name"`
				Type   string `json:"type"`
				Hook   string `json:"hook"`
				Prio   int    `json:"prio"`
				Policy string `json:"policy"`
				Handle int    `json:"handle"`
			}
			if err := json.Unmarshal(raw, &c); err == nil {
				key := c.Family + "/" + c.Table + "/" + c.Name
				chainsByKey[key] = len(out.Chains)
				out.Chains = append(out.Chains, Chain{
					Family: c.Family, Table: c.Table, Name: c.Name,
					Type: c.Type, Hook: c.Hook, Priority: c.Prio,
					Policy: c.Policy, Handle: c.Handle,
				})
			}
		}
		if raw, ok := top["rule"]; ok {
			var r struct {
				Family  string            `json:"family"`
				Table   string            `json:"table"`
				Chain   string            `json:"chain"`
				Handle  int               `json:"handle"`
				Comment string            `json:"comment"`
				Expr    []json.RawMessage `json:"expr"`
			}
			if err := json.Unmarshal(raw, &r); err == nil {
				key := r.Family + "/" + r.Table + "/" + r.Chain
				if idx, ok := chainsByKey[key]; ok {
					out.Chains[idx].RuleCount++
				}
				extractCounters(r.Family, r.Table, r.Chain, r.Handle, r.Comment, r.Expr, out)
				if r.Family == "inet" && r.Table == "miniupnpd" {
					if mapping, ok := extractUPnPMapping(r.Expr, r.Comment); ok {
						out.UPnPMappings = append(out.UPnPMappings, mapping)
					}
				}
			}
		}
	}
	return out, nil
}

func extractCounters(family, table, chain string, handle int, comment string, expr []json.RawMessage, out *Ruleset) {
	verdict := extractVerdict(expr)
	for _, e := range expr {
		var holder struct {
			Counter *struct {
				Packets int64 `json:"packets"`
				Bytes   int64 `json:"bytes"`
			} `json:"counter"`
		}
		if err := json.Unmarshal(e, &holder); err != nil {
			continue
		}
		if holder.Counter == nil {
			continue
		}
		out.Counters = append(out.Counters, Counter{
			Family: family, Table: table, ChainName: chain,
			Handle: handle, Comment: comment, Verdict: verdict,
			Packets: holder.Counter.Packets, Bytes: holder.Counter.Bytes,
		})
	}
}

// extractVerdict walks a rule's expression array and returns the
// terminating verdict keyword if one is present. Recognised verdicts
// are nft's standard terminating statements that appear as bare
// `{"<verdict>": null}` objects. Returns "" when no verdict is
// present (mangle, set-update, etc.).
func extractVerdict(expr []json.RawMessage) string {
	for _, e := range expr {
		var holder map[string]json.RawMessage
		if err := json.Unmarshal(e, &holder); err != nil {
			continue
		}
		for _, v := range []string{"drop", "accept", "return", "reject", "jump", "goto", "queue", "continue"} {
			if _, ok := holder[v]; ok {
				return v
			}
		}
	}
	return ""
}

// extractUPnPMapping walks one rule's expression array looking for
// the (proto + dport) match plus the dnat target that miniupnpd emits
// for an active port forward. Returns false when the rule isn't a
// DNAT (e.g. the chain's policy rule).
func extractUPnPMapping(expr []json.RawMessage, comment string) (UPnPMapping, bool) {
	var m UPnPMapping
	m.Description = comment
	for _, e := range expr {
		var holder map[string]json.RawMessage
		if err := json.Unmarshal(e, &holder); err != nil {
			continue
		}
		if raw, ok := holder["match"]; ok {
			var match struct {
				Op    string          `json:"op"`
				Left  json.RawMessage `json:"left"`
				Right json.RawMessage `json:"right"`
			}
			if err := json.Unmarshal(raw, &match); err == nil {
				var leftPayload struct {
					Payload struct {
						Protocol string `json:"protocol"`
						Field    string `json:"field"`
					} `json:"payload"`
				}
				if err := json.Unmarshal(match.Left, &leftPayload); err == nil {
					if leftPayload.Payload.Field == "dport" {
						m.Protocol = leftPayload.Payload.Protocol
						var port int
						if err := json.Unmarshal(match.Right, &port); err == nil {
							m.ExternalPort = port
						}
					}
				}
			}
		}
		if raw, ok := holder["dnat"]; ok {
			var dnat struct {
				Addr string `json:"addr"`
				Port int    `json:"port"`
			}
			if err := json.Unmarshal(raw, &dnat); err == nil {
				m.InternalAddr = dnat.Addr
				m.InternalPort = dnat.Port
			}
		}
	}
	if m.InternalAddr == "" || m.ExternalPort == 0 {
		return UPnPMapping{}, false
	}
	if m.InternalPort == 0 {
		m.InternalPort = m.ExternalPort
	}
	return m, true
}
