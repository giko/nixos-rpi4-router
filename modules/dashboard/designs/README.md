# Router Dashboard — Designs

Design artifacts for the SENTINEL OS router dashboard, generated via Stitch from the design brief in `docs/` (to be written). Brand is **SENTINEL OS / RPI ROUTER V2.4**, theme is "The Silent Sentinel" — dark, calm, information-dense.

## Files

| File | What it is |
|---|---|
| `design-system.md` | The full visual language spec: palette, typography, elevation, component rules |
| `01-overview.{html,png}` | Dashboard overview — hero cards, traffic graph, critical alerts, live connections |
| `02-clients.{html,png}` | Clients list — device table with route badges, top DNS consumers, traffic distribution |
| `03-vpn-pools.{html,png}` | VPN pool detail for `all_vpns` — member tunnels, flow distribution, routed clients |
| `04-adguard.{html,png}` | AdGuard DNS stats — totals, query density chart, top blocked domains, real-time query stream |

The HTML files are the designer's raw Stitch exports — Tailwind-based, self-contained. Useful as a reference for spacing, copy, and component composition. The eventual React + shadcn/ui implementation won't mirror them 1:1; it'll re-implement the same information architecture with shadcn primitives.

## Missing from this set (still to design later)

- VPN tunnels detail (single-tunnel view)
- Traffic (interface graphs + top flows)
- Firewall & PBR (rules, port forwards, nft counters)
- QoS (CAKE/HTB stats)
- System (CPU, temp, services)
- Mutation flows (v2+)
- Empty / error states
- Mobile breakpoints
