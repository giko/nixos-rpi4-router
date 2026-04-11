# Router Dashboard — Design Brief

## What it is

A web dashboard for a DIY home router built on a Raspberry Pi 4 running NixOS. The router replaces a consumer appliance: it handles DNS (AdGuard Home), DHCP, QoS, firewall/NAT, per-client WireGuard VPN steering, and a pooled multi-VPN load balancer that round-robins new flows across 4 ProtonVPN tunnels with active health checking. The dashboard is the single pane of glass for everything the router does.

## Who uses it

One technically-literate owner, on a laptop/desktop **and occasionally a phone** (while away from the couch, on the same Wi-Fi). No multi-user, no RBAC, no guest mode. Accessed only from the local network — no login screen, no auth.

## What it must answer in 2 seconds

On opening the dashboard, the owner should instantly see:

1. **Is anything broken?** (any VPN tunnel down, any service failing, any critical threshold crossed)
2. **What's my household doing right now?** (which devices are online, where are they routing, how much bandwidth is flowing)
3. **Is privacy working?** (AdGuard blocking rate, how many devices are on VPN pools, no leaks)

Everything else is drill-down.

## Content model — subsystems to cover

Eight subsystems, each deserving its own "detail" view. The designer can decide whether they're tabs, sidebar entries, drilldown pages, cards that expand inline, or something novel.

| Subsystem | Key objects | Live data |
|---|---|---|
| **VPN tunnels** | 4 WireGuard tunnels (SE, US, NL, UK) | health, handshake age, rx/tx, current exit IP, latency |
| **VPN pools** | Named pools (currently one: `all_vpns`), each containing tunnels | per-pool health, flow distribution across member tunnels, member clients |
| **Clients** | ~20-30 devices (laptops, phones, IoT, consoles) | hostname, IP, MAC, lease type (static/dynamic), current route (WAN or tunnel), allowlist status |
| **AdGuard DNS** | DNS stats | queries/hour, block rate %, top blocked domains, top clients, recent queries stream |
| **Traffic** | Network interfaces (eth0 LAN, eth1 WAN, 4x wg*) | live rx/tx rate graphs, totals, top flows |
| **Firewall & PBR** | Port forwards, nft chain counters, PBR rules (source-based, domain-based, pooled) | rule table, hit counters, recent drops, blocked-MAC hits |
| **QoS** | CAKE egress + HTB/fq_codel ingress shaping | drops, ECN marks, buffer occupancy, per-flow shaping stats |
| **System** | The Pi itself | CPU load, temperature, throttling flags, memory, zram swap, systemd service states, uptime |

## Key objects & their relationships

The dashboard has three "primary objects" a user navigates around:

- **A tunnel** (e.g. `wg_sw`) — has health, stats, exit IP, the clients currently routing through it, and flow counts.
- **A client** (e.g. `giko's phone`) — has an IP/MAC, a route (WAN or a pool/tunnel), a DNS query history, and a bandwidth usage.
- **A pool** (e.g. `all_vpns`) — has member tunnels, member clients, and a flow distribution (how new flows are spread across members).

A good design makes it obvious how to move *between* these: "show me the clients on this tunnel", "show me the tunnel this client is currently using", "show me AdGuard queries from this client". Cross-linking matters more than raw data density.

## Interaction model

- **All data is live.** The frontend polls the backend every 2–5 seconds. Counters tick, graphs slide. Designer should assume values change while being looked at.
- **Graphs** are line charts for rate data (WAN rx/tx, per-tunnel rx/tx), bar charts for distributions (flow count per pool member), sparklines for at-a-glance trends on cards.
- **Tables** are the default for list data (clients, leases, rules, top domains). Sortable, filterable, modest row counts (≤50 typical).
- **No modals for data.** Drill-down should feel like navigation, not popups. Popups OK for short confirmations later when we add mutations.
- **Mobile friendly** but not mobile-first. Desktop is primary; designer should ensure nothing catastrophically breaks on iPhone width.

## Read-only today, mutations tomorrow

v1 is **read-only**. But v2+ will add mutation actions, and the design should leave room for them without a rework. Examples of future actions:

- Toggle a client between "WAN direct" and "VPN pool"
- Add/remove a MAC from the allowlist
- Flush conntrack for a client (forces re-routing of in-flight flows)
- Restart a specific VPN tunnel
- Add a DNS rewrite
- Block a domain
- Add/remove a port forward
- Force-refresh AdGuard filter lists

So: each detail view should anticipate eventually sprouting a small actions region. Don't design corners that would fight this later.

## Aesthetic & tech constraints

- **Component library: shadcn/ui** (React + Radix + Tailwind). Designer should draw using shadcn's visual vocabulary: cards, tables, tabs, sheets, dialogs, badges, tooltips, charts via Recharts. No exotic bespoke components — if shadcn doesn't offer it, we don't want it in v1.
- **Dark-first.** Router admins live in dark mode. Light mode is acceptable as a toggle, but design everything dark first.
- **Information-dense but calm.** Tailwind-default radiuses and spacing are fine. Prefer subdued colors with semantic accents (green = healthy, amber = degraded, red = failed) over a saturated rainbow.
- **No marketing surface.** No hero banners, no onboarding flow, no empty-state illustrations, no dashboards-for-the-sake-of-dashboards. Every pixel earns its keep.
- **Monospace for IPs, MACs, hex marks.** Tabular numerals (`font-variant-numeric: tabular-nums`) for counters so they don't jitter.

## What we need from the designer

1. **Home/overview layout.** The first screen. What the owner sees when they open the dashboard. Should answer "is anything broken + what's flowing" in 2 seconds.
2. **At least 3 detail views** — our pick: **VPN pools**, **Clients**, **AdGuard**. These are the most data-dense and cross-linked; nailing their shape unblocks the others by pattern.
3. **Empty & error states** for each — what a page looks like when the underlying service is down or has no data.
4. **A visual language** for object types (tunnel, client, pool) that we can reuse everywhere. Badge shapes, status dots, iconography.
5. **Navigation shape** — how the user moves between the 8 subsystems, whether sidebar / tabs / tile grid / something else. Designer's call.
6. **(Optional) one mutation flow** — e.g., "move a client to a different pool" — to validate that the design leaves room for actions without reworking.

**Deliverables** can be Figma, Excalidraw, Penpot, or hand-sketched PDFs — whatever is fastest for them. We'll convert to code.

## Things to ignore

- Authentication, login, user management (LAN-only, no auth)
- Multi-tenancy, branding, logos
- Internationalization (English only)
- Mobile as a primary device
- Animations beyond "new value glows briefly as it updates"
