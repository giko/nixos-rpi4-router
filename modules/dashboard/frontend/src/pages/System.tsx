import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Service } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatPercent, formatDuration, formatAbsoluteTime } from "@/lib/formatters";

function StatTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "healthy" | "degraded" | "failed" | undefined;
}) {
  const colorClass =
    tone === "failed"
      ? "text-rose"
      : tone === "degraded"
        ? "text-amber"
        : tone === "healthy"
          ? "text-emerald"
          : "";
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className={`text-lg font-semibold ${colorClass}`}>
        {value}
      </MonoText>
    </div>
  );
}

const serviceColumns: Column<Service>[] = [
  {
    key: "name",
    label: "Service",
    render: (r) => <MonoText className="text-xs">{r.name}</MonoText>,
    sortValue: (r) => r.name,
  },
  {
    key: "active",
    label: "Status",
    render: (r) => (
      <StatusBadge kind={r.active ? "healthy" : "failed"}>
        {r.raw_state}
      </StatusBadge>
    ),
    sortValue: (r) => (r.active ? 0 : 1),
  },
];

export function System() {
  const systemQ = useQuery({
    queryKey: queryKeys.system(),
    queryFn: api.system,
    refetchInterval: 2_000,
  });

  if (systemQ.isError && !systemQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load system data — retry shortly.
      </div>
    );
  }
  if (systemQ.isPending || !systemQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const sys = systemQ.data.data;
  const refetchFailed = systemQ.isError;
  const cpuActive = sys.cpu.percent_user + sys.cpu.percent_system;
  const memTone =
    sys.memory.percent_used >= 90
      ? "failed"
      : sys.memory.percent_used >= 75
        ? "degraded"
        : "healthy";
  const tempTone =
    sys.temperature_c >= 80
      ? "failed"
      : sys.temperature_c >= 70
        ? "degraded"
        : "healthy";
  const throttleTone = sys.throttled_flag ? "failed" : "healthy";
  const services = sys.services ?? [];
  const downServices = services.filter((s) => !s.active);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">System</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={systemQ.data.stale ?? false}
            updatedAt={systemQ.data.updated_at ?? null}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatTile label="CPU active" value={formatPercent(cpuActive)} />
        <StatTile
          label="Memory used"
          value={`${formatPercent(sys.memory.percent_used)} (${formatBytes(sys.memory.used_bytes)} / ${formatBytes(sys.memory.total_bytes)})`}
          tone={memTone}
        />
        <StatTile
          label="Temperature"
          value={`${sys.temperature_c.toFixed(1)} °C`}
          tone={tempTone}
        />
        <StatTile
          label="Throttle"
          value={sys.throttled_flag ? "ACTIVE" : "clear"}
          tone={throttleTone}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">CPU breakdown</p>
          <div className="text-xs space-y-1 mt-2 font-mono">
            <div className="flex justify-between">
              <span>user</span>
              <MonoText>{formatPercent(sys.cpu.percent_user)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>system</span>
              <MonoText>{formatPercent(sys.cpu.percent_system)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>iowait</span>
              <MonoText>{formatPercent(sys.cpu.percent_iowait)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>idle</span>
              <MonoText>{formatPercent(sys.cpu.percent_idle)}</MonoText>
            </div>
          </div>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">Throttle reason</p>
          <MonoText className="text-xs break-all text-on-surface-variant">
            {sys.throttled || "—"}
          </MonoText>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">Uptime</p>
          <MonoText className="text-sm">
            {formatDuration(sys.uptime_seconds)}
          </MonoText>
          <p className="label-xs mt-3 mb-1">Boot time</p>
          <MonoText className="text-xs text-on-surface-variant">
            {formatAbsoluteTime(sys.boot_time)}
          </MonoText>
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <h2 className="label-xs">Services</h2>
          {downServices.length > 0 && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              {downServices.length} down
            </span>
          )}
        </div>
        {services.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No services tracked.
          </p>
        ) : (
          <DataTable
            columns={serviceColumns}
            rows={services}
            rowKey={(r) => r.name}
          />
        )}
      </div>
    </div>
  );
}
