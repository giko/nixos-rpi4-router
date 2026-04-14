import { useCallback, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { WifiOff } from "lucide-react";
import { api } from "@/lib/api";
import type { ClientTopDestination } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { cn } from "@/lib/utils";
import { formatBytes } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";
import { useClientDetailFilter } from "./useClientDetailFilter";

/* ------------------------------------------------------------------ */
/*  Sortable header                                                    */
/* ------------------------------------------------------------------ */

type SortKey = "domain" | "queries" | "blocked" | "bytes" | "flows";
type SortDir = "asc" | "desc";

function HeaderCell({
  label,
  sortKey,
  currentKey,
  currentDir,
  onSort,
  align = "left",
}: {
  label: string;
  sortKey: SortKey;
  currentKey: SortKey;
  currentDir: SortDir;
  onSort: (k: SortKey) => void;
  align?: "left" | "right";
}) {
  const active = currentKey === sortKey;
  return (
    <th
      className={cn(
        "label-xs h-8 px-3 cursor-pointer select-none",
        align === "right" ? "text-right" : "text-left",
      )}
      onClick={() => onSort(sortKey)}
    >
      {label}
      {active ? (currentDir === "asc" ? " \u2191" : " \u2193") : ""}
    </th>
  );
}

/* ------------------------------------------------------------------ */
/*  Panel                                                              */
/* ------------------------------------------------------------------ */

export function TopDestinations({ ip }: { ip: string }) {
  const [sortKey, setSortKey] = useState<SortKey>("queries");
  const [sortDir, setSortDir] = useState<SortDir>("desc");

  const { selectedDomain, toggleDomain } = useClientDetailFilter();

  const q = useQuery({
    queryKey: queryKeys.clientTopDestinations(ip),
    queryFn: () => api.clientTopDestinations(ip),
    enabled: !!ip,
    refetchInterval: 3_000,
    retry: (count, error) => {
      if (error instanceof Error && error.message.includes("404")) return false;
      return count < 3;
    },
  });

  const onSort = useCallback((k: SortKey) => {
    setSortKey((prev) => {
      if (prev === k) {
        setSortDir((d) => (d === "asc" ? "desc" : "asc"));
        return prev;
      }
      setSortDir(k === "domain" ? "asc" : "desc");
      return k;
    });
  }, []);

  const rows: ClientTopDestination[] = q.data?.data.destinations ?? [];
  const sorted = useMemo(() => {
    const dir = sortDir === "asc" ? 1 : -1;
    return [...rows].sort((a, b) => {
      const av: string | number =
        sortKey === "domain" ? a.domain.toLowerCase() : (a[sortKey] as number);
      const bv: string | number =
        sortKey === "domain" ? b.domain.toLowerCase() : (b[sortKey] as number);
      return av < bv ? -dir : av > bv ? dir : 0;
    });
  }, [rows, sortKey, sortDir]);

  if (q.isError && !q.data) return null;
  if (q.isPending || !q.data) {
    return (
      <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant">
        Loading destinations...
      </div>
    );
  }

  const data = q.data.data;

  if (data.destinations === null) {
    return (
      <div className="bg-surface-container rounded-sm p-4 flex items-center gap-2 text-xs text-on-surface-variant">
        <WifiOff size={14} strokeWidth={1.5} />
        <span>
          Per-client accounting unavailable for static leases.
        </span>
      </div>
    );
  }

  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="label-xs">Top destinations</p>
        {selectedDomain && (
          <button
            onClick={() => toggleDomain(selectedDomain)}
            className="text-[10px] uppercase tracking-wider font-bold text-primary hover:underline"
          >
            Clear filter
          </button>
        )}
      </div>

      {sorted.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono py-2">
          No tracked destinations yet.
        </p>
      ) : (
        <div className="relative w-full overflow-auto">
          <table className="w-full caption-bottom text-sm">
            <thead>
              <tr className="bg-surface-low">
                <HeaderCell
                  label="Domain"
                  sortKey="domain"
                  currentKey={sortKey}
                  currentDir={sortDir}
                  onSort={onSort}
                />
                <HeaderCell
                  label="Queries"
                  sortKey="queries"
                  currentKey={sortKey}
                  currentDir={sortDir}
                  onSort={onSort}
                  align="right"
                />
                <HeaderCell
                  label="Blocked"
                  sortKey="blocked"
                  currentKey={sortKey}
                  currentDir={sortDir}
                  onSort={onSort}
                  align="right"
                />
                <HeaderCell
                  label="Bytes"
                  sortKey="bytes"
                  currentKey={sortKey}
                  currentDir={sortDir}
                  onSort={onSort}
                  align="right"
                />
                <HeaderCell
                  label="Flows"
                  sortKey="flows"
                  currentKey={sortKey}
                  currentDir={sortDir}
                  onSort={onSort}
                  align="right"
                />
              </tr>
            </thead>
            <tbody>
              {sorted.map((d) => {
                const selected = selectedDomain === d.domain;
                return (
                  <tr
                    key={d.domain}
                    onClick={() => toggleDomain(d.domain)}
                    className={cn(
                      "cursor-pointer transition-colors",
                      selected
                        ? "bg-primary/10"
                        : "hover:bg-surface-high/50",
                    )}
                  >
                    <td className="px-3 py-2">
                      <MonoText className="text-xs">{d.domain}</MonoText>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <MonoText className="text-xs">
                        {d.queries.toLocaleString()}
                      </MonoText>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <MonoText
                        className={cn(
                          "text-xs",
                          d.blocked > 0 ? "text-rose" : "text-on-surface-variant",
                        )}
                      >
                        {d.blocked.toLocaleString()}
                      </MonoText>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <MonoText className="text-xs">
                        {formatBytes(d.bytes)}
                      </MonoText>
                    </td>
                    <td className="px-3 py-2 text-right">
                      <MonoText className="text-xs">
                        {d.flows.toLocaleString()}
                      </MonoText>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
