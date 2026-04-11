# Design System Strategy: The Observational Command

> Source: Stitch project `nixos router` (ID `5465859339248317807`), generated 2026-04-11.
> Brand: **SENTINEL OS** / `RPI ROUTER V2.4`.

## 1. Overview & Creative North Star: "The Silent Sentinel"

The design system for this Raspberry Pi router interface is built on the Creative North Star of **"The Silent Sentinel."** Unlike consumer-grade networking tools that scream for attention, this system is a high-performance instrument. It treats data as an editorial asset — clean, quiet, and profoundly dense.

We move beyond the "template" look of standard utility dashboards by embracing **Intentional Asymmetry**. By pairing rigid, monospaced technical data with expansive, airy Inter headings, we create a rhythmic contrast. We prioritize "Calm Density" — a philosophy where information is packed tightly but remains legible through strict tonal layering rather than physical boundaries.

## 2. Colors & Surface Logic

Dark-mode first. Depth is simulated by light absorption, not light reflection.

### Palette (named tokens)

| Token | Hex | Usage |
|---|---|---|
| `surface` | `#0e0e0e` | Base canvas |
| `surface_container_lowest` | `#000000` | Inset wells (logs, inputs) |
| `surface_container_low` | `#131313` | Large layout blocks / sidebar |
| `surface_container` | `#191a1a` | Cards, modules |
| `surface_container_high` | `#1f2020` | Actionable cards |
| `surface_container_highest` | `#252626` | Active / hover lift |
| `surface_variant` | `#252626` | Glass panels (60-70% + blur) |
| `primary` | `#c0c1ff` | Primary CTAs, focus |
| `primary_container` | `#2f2ebe` | Gradient partner with primary |
| `on_surface` | `#e7e5e4` | Brightest text — never pure white |
| `on_surface_variant` | `#acabaa` | Muted metadata |
| `outline_variant` | `#484848` | Ghost borders @ 15% opacity only |

Status accents:
- **Healthy / Connected** — emerald (success)
- **Warning / Degraded** — amber (warning)
- **Critical / Down** — rose (error `#ec7c8a`)
- **Info / VPN tunnels** — `#80d1ff`

### The "No-Line" Rule

Traditional dashboards use borders to solve layout problems. This design system prohibits 1px solid borders for sectioning. **Boundaries must be defined solely through background color shifts.** If two sections meet, they are distinguished by the transition from `surface` to `surface_container_low`. This creates a seamless, "molded" appearance.

### The "Glass & Gradient" Rule

For floating elements like tooltips or dropdowns, use **Glassmorphism**. Apply `surface_variant` with a 70% opacity and a `backdrop-blur` of 12px. To provide "soul" to the Brand/System accent, use a subtle linear gradient from `primary` (`#c0c1ff`) to `primary_container` (`#2f2ebe`) for high-level status indicators or main CTAs.

## 3. Typography: The Technical Editorial

Dual-font strategy to separate "Instruction" from "Raw Data."

- **UI / Instructional — Inter**
  - `display-sm` (2.25rem) — Reserved for critical metrics (e.g., total uptime).
  - `title-sm` (1rem) — Standard for module headers.
  - `label-sm` (0.6875rem) — Metadata, always in `on_surface_variant` for a muted feel.
- **Technical Data — JetBrains Mono / Fira Code**
  - Every IP, MAC, hex fwmark, and throughput stat (Kbps/Mbps) uses the mono scale. Ensures tabular data aligns perfectly and feels like a professional terminal.
  - Use tabular numerals (`font-variant-numeric: tabular-nums`) so counters don't jitter as values update.

## 4. Elevation & Depth

Elevation is felt, not seen.

- **Layering Principle:** Stacking is the primary tool. A `surface_container_highest` widget sits inside a `surface_container_low` sidebar, creating a natural "lift."
- **Ambient Shadows:** Only for floating modals.
  ```
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.5);
  ```
- **Ghost Border Fallback:** If a border is required for accessibility on a button or input, use `outline_variant` at **15% opacity**. A 100% opaque border is considered a failure of the layout.

## 5. Components

### Buttons & Interaction

- **Primary** — Gradient background (`primary` → `primary_dim`), `on_primary` text. No border.
- **Secondary** — `surface_container_highest` background, subtle `on_surface` text.
- **Ghost / Tertiary** — No background. Only `on_surface_variant` text that shifts to `on_surface` on hover.

### High-Density Lists (Traffic / Logs)

- **Divider Rule:** No divider lines. Separate log entries using 4px vertical whitespace or a subtle toggle between `surface` and `surface_container_low` for zebra-striping.
- **Monospace Toggles:** Technical switches use `sm` (0.125rem) or `md` (0.375rem) corner radii to maintain a "hardware" feel.

### Specialized Router Components

- **Throughput Sparklines:** `primary` for system traffic, `info` (`#80d1ff`) for VPN tunnels. 1.5px stroke, no fill.
- **Status Pips:** 6px circles.
  - Healthy → emerald
  - Latency → amber
  - Down → rose

## 6. Do's and Don'ts

### Do

- Use `surface_container_lowest` (`#000000`) for "well" effects (inset areas where logs reside).
- Use `label-sm` in ALL CAPS for sidebar category headers to increase the "pro-tool" aesthetic.
- Maintain a strict 4px/8px spacing grid so density feels organized rather than cluttered.

### Don't

- Don't use pure white text. Brightest text is `on_surface` (`#e7e5e4`) to reduce eye strain.
- Don't use standard `<select>` dropdowns. Use custom, low-profile popovers with the Glassmorphism rule.
- Don't put shadows on cards that are part of the main grid — let background shifts do the work.
