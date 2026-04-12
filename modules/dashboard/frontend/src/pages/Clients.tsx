import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { cn } from "@/lib/utils";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { PoolBadge } from "@/components/PoolBadge";
import { ClientBadge } from "@/components/ClientBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";

/* ------------------------------------------------------------------ */
/*  Filter types & logic                                               */
/* ------------------------------------------------------------------ */

type Filter = "all" | "vpn" | "wan" | "static" | "unknown";

const FILTERS: { key: Filter; label: string }[] = [
  { key: "all", label: "All Devices" },
  { key: "vpn", label: "On VPN" },
  { key: "wan", label: "Direct to WAN" },
  { key: "static", label: "Static Leases" },
  { key: "unknown", label: "Unknown" },
];

function matchesFilter(c: Client, filter: Filter): boolean {
  switch (filter) {
    case "all":
      return true;
    case "vpn":
      return c.route.startsWith("pool:") || c.route.startsWith("tunnel:");
    case "wan":
      return c.route === "wan";
    case "static":
      return c.lease_type === "static";
    case "unknown":
      return (
        c.lease_type === "neighbor" ||
        c.hostname === "" ||
        c.allowlist_status === "blocked"
      );
  }
}

/* ------------------------------------------------------------------ */
/*  Status pip                                                         */
/* ------------------------------------------------------------------ */

function StatusPip({ client }: { client: Client }) {
  const color =
    client.allowlist_status === "blocked"
      ? "bg-rose"
      : client.allowlist_status === "allowed"
        ? "bg-emerald"
        : "bg-amber";
  return <span className={cn("inline-block h-1.5 w-1.5 rounded-full", color)} />;
}

/* ------------------------------------------------------------------ */
/*  Route cell                                                         */
/* ------------------------------------------------------------------ */

function RouteCell({ route }: { route: string }) {
  if (route.startsWith("pool:")) {
    return <PoolBadge name={route.slice(5)} />;
  }
  if (route.startsWith("tunnel:")) {
    return <StatusBadge kind="info">{route.slice(7)}</StatusBadge>;
  }
  if (route === "wan") {
    return <StatusBadge kind="muted">WAN DIRECT</StatusBadge>;
  }
  return <StatusBadge kind="muted">{route || "--"}</StatusBadge>;
}

/* ------------------------------------------------------------------ */
/*  Allowlist cell                                                     */
/* ------------------------------------------------------------------ */

function AllowlistCell({ status }: { status: string }) {
  const kind =
    status === "allowed"
      ? "healthy"
      : status === "blocked"
        ? "failed"
        : "degraded";
  return <StatusBadge kind={kind}>{status || "unknown"}</StatusBadge>;
}

/* ------------------------------------------------------------------ */
/*  Table columns                                                      */
/* ------------------------------------------------------------------ */

const columns: Column<Client>[] = [
  {
    key: "hostname",
    label: "Hostname",
    render: (r) => (
      <div className="flex items-center gap-2">
        <StatusPip client={r} />
        <ClientBadge
          ip={r.ip}
          label={r.hostname || undefined}
        />
        {!r.hostname && (
          <span className="text-on-surface-variant text-xs italic">--</span>
        )}
      </div>
    ),
    sortValue: (r) => (r.hostname || r.ip).toLowerCase(),
  },
  {
    key: "ip",
    label: "IP Address",
    render: (r) => <MonoText className="text-xs">{r.ip}</MonoText>,
    sortValue: (r) =>
      r.ip
        .split(".")
        .map((o) => o.padStart(3, "0"))
        .join("."),
  },
  {
    key: "mac",
    label: "MAC Address",
    render: (r) => (
      <MonoText className="text-[10px] text-on-surface-variant">
        {r.mac || "--"}
      </MonoText>
    ),
    sortValue: (r) => r.mac,
  },
  {
    key: "route",
    label: "Active Route",
    render: (r) => <RouteCell route={r.route} />,
    sortValue: (r) => r.route,
  },
  {
    key: "allowlist",
    label: "Allowlist",
    render: (r) => <AllowlistCell status={r.allowlist_status} />,
    sortValue: (r) => r.allowlist_status,
  },
  {
    key: "conns",
    label: "Conns",
    render: (r) => <MonoText>{r.conn_count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.conn_count,
    className: "text-right",
  },
];

/* ------------------------------------------------------------------ */
/*  Clients page                                                       */
/* ------------------------------------------------------------------ */

export function Clients() {
  const [filter, setFilter] = useState<Filter>("all");
  const [search, setSearch] = useState("");

  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 5_000,
  });

  const clients = clientsQ.data?.data.clients ?? [];

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    return clients
      .filter((c) => matchesFilter(c, filter))
      .filter(
        (c) =>
          !q ||
          c.hostname.toLowerCase().includes(q) ||
          c.ip.toLowerCase().includes(q) ||
          c.mac.toLowerCase().includes(q),
      );
  }, [clients, filter, search]);

  if (clientsQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Clients</h1>
        <StaleIndicator
          stale={clientsQ.data?.stale ?? false}
          updatedAt={clientsQ.data?.updated_at ?? null}
        />
      </div>

      {/* Filter bar + search */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-2 overflow-x-auto">
          {FILTERS.map((f) => (
            <button
              key={f.key}
              onClick={() => setFilter(f.key)}
              className={cn(
                "px-4 py-1.5 rounded-sm text-[10px] font-bold uppercase tracking-wider transition-colors whitespace-nowrap",
                filter === f.key
                  ? "bg-primary text-on-primary"
                  : "bg-surface-container hover:bg-surface-high text-on-surface-variant",
              )}
            >
              {f.label}
            </button>
          ))}
        </div>
        <div className="relative min-w-[280px]">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by name, IP, or MAC..."
            className="w-full bg-surface-container-low border-none focus:ring-1 focus:ring-primary/30 rounded-sm py-2 px-3 text-xs text-on-surface placeholder:text-on-surface-variant/50 font-mono"
          />
        </div>
      </div>

      {/* Table */}
      {filtered.length === 0 ? (
        <p className="text-sm text-on-surface-variant font-mono">
          No clients match the current filters.
        </p>
      ) : (
        <DataTable
          columns={columns}
          rows={filtered}
          rowKey={(r) => r.ip}
        />
      )}
    </div>
  );
}
