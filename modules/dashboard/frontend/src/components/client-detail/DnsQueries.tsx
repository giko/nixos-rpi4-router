import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { ClientDnsQuery } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { cn } from "@/lib/utils";
import { formatAbsoluteTime } from "@/lib/formatters";
import { MonoText } from "@/components/MonoText";
import { useClientDetailFilter } from "./useClientDetailFilter";

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function formatElapsed(ms: number): string {
  if (!Number.isFinite(ms)) return "--";
  if (ms < 1) return `${ms.toFixed(2)} ms`;
  if (ms < 100) return `${ms.toFixed(1)} ms`;
  return `${Math.round(ms)} ms`;
}

function matchesDomainFilter(question: string, selected: string | null): boolean {
  if (!selected) return false;
  if (question === selected) return true;
  return question.endsWith("." + selected);
}

/* ------------------------------------------------------------------ */
/*  Row                                                                */
/* ------------------------------------------------------------------ */

function QueryRow({
  entry,
  highlighted,
}: {
  entry: ClientDnsQuery;
  highlighted: boolean;
}) {
  return (
    <tr
      className={cn(
        entry.blocked ? "bg-rose/10" : undefined,
        highlighted && "ring-1 ring-primary/40",
      )}
    >
      <td className="px-3 py-1.5 whitespace-nowrap">
        <MonoText className="text-[10px] text-on-surface-variant">
          {formatAbsoluteTime(entry.time)}
        </MonoText>
      </td>
      <td className="px-3 py-1.5">
        <MonoText className="text-xs">{entry.question || "--"}</MonoText>
      </td>
      <td className="px-3 py-1.5">
        <MonoText className="text-[10px] text-on-surface-variant">
          {entry.question_type || "--"}
        </MonoText>
      </td>
      <td className="px-3 py-1.5">
        <span
          className={cn(
            "text-[10px] font-bold uppercase tracking-wider",
            entry.blocked ? "text-rose" : "text-emerald",
          )}
        >
          {entry.reason || (entry.blocked ? "blocked" : "allowed")}
        </span>
      </td>
      <td className="px-3 py-1.5 text-right">
        <MonoText className="text-[10px] text-on-surface-variant">
          {formatElapsed(entry.elapsed_ms)}
        </MonoText>
      </td>
      <td className="px-3 py-1.5">
        <MonoText className="text-[10px] text-on-surface-variant truncate">
          {entry.upstream || "--"}
        </MonoText>
      </td>
    </tr>
  );
}

/* ------------------------------------------------------------------ */
/*  Panel                                                              */
/* ------------------------------------------------------------------ */

export function DnsQueries({ ip }: { ip: string }) {
  const { selectedDomain } = useClientDetailFilter();

  const q = useQuery({
    queryKey: queryKeys.clientDns(ip),
    queryFn: () => api.clientDns(ip),
    enabled: !!ip,
    refetchInterval: 3_000,
    retry: (count, error) => {
      if (error instanceof Error && error.message.includes("404")) return false;
      return count < 3;
    },
  });

  if (q.isError && !q.data) return null;
  if (q.isPending || !q.data) {
    return (
      <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant">
        Loading DNS queries...
      </div>
    );
  }

  const data = q.data.data;
  const entries: ClientDnsQuery[] = data.recent ?? [];

  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <p className="label-xs">DNS queries</p>
        <MonoText className="text-[10px] text-on-surface-variant">
          {data.count.toLocaleString()} / {data.limit.toLocaleString()}
        </MonoText>
      </div>

      {entries.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono py-2">
          No recent DNS queries.
        </p>
      ) : (
        <div className="relative w-full overflow-auto max-h-[420px]">
          <table className="w-full caption-bottom text-sm">
            <thead className="sticky top-0 bg-surface-low">
              <tr>
                <th className="label-xs h-8 px-3 text-left">Time</th>
                <th className="label-xs h-8 px-3 text-left">Domain</th>
                <th className="label-xs h-8 px-3 text-left">Type</th>
                <th className="label-xs h-8 px-3 text-left">Reason</th>
                <th className="label-xs h-8 px-3 text-right">Elapsed</th>
                <th className="label-xs h-8 px-3 text-left">Upstream</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry, i) => (
                <QueryRow
                  key={`${entry.time}-${i}`}
                  entry={entry}
                  highlighted={matchesDomainFilter(entry.question, selectedDomain)}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
