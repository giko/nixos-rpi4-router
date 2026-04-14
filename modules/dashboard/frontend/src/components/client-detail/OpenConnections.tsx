import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowDownLeft, ArrowUpRight } from "lucide-react";
import { api } from "@/lib/api";
import type { ClientFlow } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { cn } from "@/lib/utils";
import { formatBytes } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { useClientDetailFilter } from "./useClientDetailFilter";

/* ------------------------------------------------------------------ */
/*  Constants                                                          */
/* ------------------------------------------------------------------ */

// Hard cap on rendered rows. The /connections endpoint is naturally
// bounded by an active client's open flows, but we keep a ceiling so a
// pathological conntrack dump can't freeze the browser.
const MAX_ROWS = 500;

/* ------------------------------------------------------------------ */
/*  Route badge                                                        */
/* ------------------------------------------------------------------ */

function RouteBadge({ tag }: { tag: string }) {
  if (!tag) return <StatusBadge kind="muted">--</StatusBadge>;
  if (tag.startsWith("pool:")) {
    return <StatusBadge kind="info">{tag.slice(5)}</StatusBadge>;
  }
  if (tag.startsWith("tunnel:")) {
    return <StatusBadge kind="info">{tag.slice(7)}</StatusBadge>;
  }
  if (tag === "wan") return <StatusBadge kind="muted">WAN</StatusBadge>;
  return <StatusBadge kind="muted">{tag}</StatusBadge>;
}

/* ------------------------------------------------------------------ */
/*  Filter chips                                                       */
/* ------------------------------------------------------------------ */

function Chip({
  label,
  active,
  onClick,
}: {
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "px-2 py-0.5 rounded-sm text-[10px] font-bold uppercase tracking-wider transition-colors",
        active
          ? "bg-primary text-on-primary"
          : "bg-surface-high hover:bg-surface-highest text-on-surface-variant",
      )}
    >
      {label}
    </button>
  );
}

/* ------------------------------------------------------------------ */
/*  Panel                                                              */
/* ------------------------------------------------------------------ */

export function OpenConnections({ ip }: { ip: string }) {
  const [routeFilter, setRouteFilter] = useState<string | null>(null);
  const [protoFilter, setProtoFilter] = useState<string | null>(null);

  const { toggleDomain } = useClientDetailFilter();

  const q = useQuery({
    queryKey: queryKeys.clientConnections(ip),
    queryFn: () => api.clientConnections(ip),
    enabled: !!ip,
    refetchInterval: 3_000,
    retry: (count, error) => {
      if (error instanceof Error && error.message.includes("404")) return false;
      return count < 3;
    },
  });

  const flows: ClientFlow[] = q.data?.data.flows ?? [];

  const routeTags = useMemo(() => {
    const set = new Set<string>();
    for (const f of flows) if (f.route_tag) set.add(f.route_tag);
    return Array.from(set).sort();
  }, [flows]);

  const protos = useMemo(() => {
    const set = new Set<string>();
    for (const f of flows) if (f.proto) set.add(f.proto);
    return Array.from(set).sort();
  }, [flows]);

  const filtered = useMemo(() => {
    return flows.filter((f) => {
      if (routeFilter && f.route_tag !== routeFilter) return false;
      if (protoFilter && f.proto !== protoFilter) return false;
      return true;
    });
  }, [flows, routeFilter, protoFilter]);

  const toggleRoute = (tag: string) =>
    setRouteFilter((cur) => (cur === tag ? null : tag));
  const toggleProto = (p: string) =>
    setProtoFilter((cur) => (cur === p ? null : p));

  if (q.isError && !q.data) return null;
  if (q.isPending || !q.data) {
    return (
      <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant">
        Loading connections...
      </div>
    );
  }

  const data = q.data.data;
  const truncated = filtered.length > MAX_ROWS;
  const visible = truncated ? filtered.slice(0, MAX_ROWS) : filtered;

  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="label-xs">Open connections</p>
        <MonoText className="text-[10px] text-on-surface-variant">
          {data.count.toLocaleString()} flows
        </MonoText>
      </div>

      {(routeTags.length > 0 || protos.length > 0) && (
        <div className="flex flex-wrap items-center gap-3">
          {routeTags.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="label-xs mr-1">Route</span>
              {routeTags.map((t) => (
                <Chip
                  key={t}
                  label={t}
                  active={routeFilter === t}
                  onClick={() => toggleRoute(t)}
                />
              ))}
            </div>
          )}
          {protos.length > 0 && (
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="label-xs mr-1">Proto</span>
              {protos.map((p) => (
                <Chip
                  key={p}
                  label={p}
                  active={protoFilter === p}
                  onClick={() => toggleProto(p)}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {visible.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono py-2">
          {flows.length === 0
            ? "No open connections."
            : "No flows match the current filters."}
        </p>
      ) : (
        <div className="relative w-full overflow-auto max-h-[480px]">
          <table className="w-full caption-bottom text-sm">
            <thead className="sticky top-0 bg-surface-low">
              <tr>
                <th className="label-xs h-8 px-3 text-left">Proto</th>
                <th className="label-xs h-8 px-3 text-left">Dir</th>
                <th className="label-xs h-8 px-3 text-left">Dest</th>
                <th className="label-xs h-8 px-3 text-right">Port</th>
                <th className="label-xs h-8 px-3 text-left">Route</th>
                <th className="label-xs h-8 px-3 text-right">Bytes {"\u2193/\u2191"}</th>
                <th className="label-xs h-8 px-3 text-left">State</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((f, i) => {
                const Arrow = f.direction === "outbound" ? ArrowUpRight : ArrowDownLeft;
                const clickable = !!f.domain;
                return (
                  <tr
                    key={`${f.proto}-${f.local_ip}-${f.local_port}-${f.remote_ip}-${f.remote_port}-${i}`}
                    onClick={
                      clickable ? () => toggleDomain(f.domain) : undefined
                    }
                    className={cn(
                      clickable && "cursor-pointer hover:bg-surface-high/50",
                    )}
                  >
                    <td className="px-3 py-1.5">
                      <MonoText className="text-xs uppercase">
                        {f.proto || "--"}
                      </MonoText>
                    </td>
                    <td className="px-3 py-1.5">
                      <span
                        title={f.direction}
                        className="inline-flex items-center text-on-surface-variant"
                      >
                        <Arrow size={14} strokeWidth={1.5} />
                      </span>
                    </td>
                    <td className="px-3 py-1.5">
                      {f.domain ? (
                        <div className="flex flex-col leading-tight">
                          <MonoText className="text-xs">{f.domain}</MonoText>
                          <MonoText className="text-[10px] text-on-surface-variant">
                            {f.remote_ip || "--"}
                          </MonoText>
                        </div>
                      ) : (
                        <MonoText className="text-xs">
                          {f.remote_ip || "--"}
                        </MonoText>
                      )}
                    </td>
                    <td className="px-3 py-1.5 text-right">
                      <MonoText className="text-xs">
                        {f.remote_port || "--"}
                      </MonoText>
                    </td>
                    <td className="px-3 py-1.5">
                      <RouteBadge tag={f.route_tag} />
                    </td>
                    <td className="px-3 py-1.5 text-right">
                      <MonoText className="text-[10px]">
                        <span className="text-emerald">
                          {formatBytes(f.client_rx_bytes)}
                        </span>
                        <span className="mx-1 text-on-surface-variant">
                          {"\u00B7"}
                        </span>
                        <span className="text-info">
                          {formatBytes(f.client_tx_bytes)}
                        </span>
                      </MonoText>
                    </td>
                    <td className="px-3 py-1.5">
                      <MonoText className="text-[10px] text-on-surface-variant">
                        {f.state || "--"}
                      </MonoText>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {truncated && (
            <p className="text-[10px] text-on-surface-variant font-mono px-3 py-2">
              Showing first {MAX_ROWS.toLocaleString()} of {filtered.length.toLocaleString()} flows.
            </p>
          )}
        </div>
      )}
    </div>
  );
}
