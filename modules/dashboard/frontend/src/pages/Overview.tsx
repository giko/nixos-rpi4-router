import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Pool, System, Traffic, Tunnel, AdguardStats, Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import {
  formatBps,
  formatDuration,
  formatPercent,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { MonoText } from "@/components/MonoText";
import { Sparkline } from "@/components/Sparkline";
import { StaleIndicator } from "@/components/StaleIndicator";

/* ------------------------------------------------------------------ */
/*  HealthStrip                                                       */
/* ------------------------------------------------------------------ */

type PipColor = "emerald" | "amber" | "rose";

function HealthCard({
  label,
  color,
  value,
  detail,
}: {
  label: string;
  color: PipColor;
  value: string;
  detail: string;
}) {
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-2">{label}</p>
      <div className="flex items-center gap-2 mb-1">
        <span
          className={cn(
            "inline-block h-2 w-2 rounded-full",
            color === "emerald" && "bg-emerald",
            color === "amber" && "bg-amber",
            color === "rose" && "bg-rose",
          )}
        />
        <MonoText className="text-lg font-semibold">{value}</MonoText>
      </div>
      <MonoText className="text-xs text-on-surface-variant">{detail}</MonoText>
    </div>
  );
}

function serviceColor(
  services: System["services"],
  name: string,
): PipColor {
  if (!services) return "amber";
  const svc = services.find((s) => s.name === name);
  if (!svc) return "amber";
  return svc.active ? "emerald" : "rose";
}

function HealthStrip({
  system,
  pools,
  wanOperstate,
}: {
  system: System | undefined;
  pools: Pool[] | undefined;
  wanOperstate: string | undefined; // from traffic.interfaces[eth1].operstate
}) {
  // WAN status: based on physical link state (operstate), not nftables service
  const wanUp = wanOperstate === "up";
  const wanColor: PipColor = wanOperstate === undefined ? "amber" : wanUp ? "emerald" : "rose";
  const wanValue = wanOperstate === undefined ? "..." : wanUp ? "ONLINE" : "DOWN";
  const wanDetail = system
    ? wanUp
      ? `uptime ${formatDuration(system.uptime_seconds)}`
      : `link ${wanOperstate ?? "unknown"}`
    : "loading...";

  // DNS status: service running AND WAN link up → resolving.
  // Service running but WAN down → degraded (can serve cache only).
  const dnsActive = serviceColor(system?.services ?? null, "adguardhome.service") === "emerald";
  const dnsColor: PipColor = !system ? "amber" : dnsActive && wanUp ? "emerald" : dnsActive && !wanUp ? "amber" : "rose";
  const dnsValue = !system ? "..." : dnsActive && wanUp ? "RESOLVING" : dnsActive && !wanUp ? "DEGRADED" : "DOWN";
  const dnsDetail = !system
    ? "loading..."
    : dnsActive && !wanUp
      ? "cache only — no WAN"
      : dnsActive
        ? "adguardhome active"
        : "adguardhome inactive";

  let poolOnline = 0;
  let poolTotal = 0;
  if (pools) {
    for (const p of pools) {
      for (const m of p.members) {
        poolTotal++;
        if (m.healthy) poolOnline++;
      }
    }
  }
  const poolColor: PipColor =
    poolTotal === 0
      ? "amber"
      : poolOnline === poolTotal
        ? "emerald"
        : poolOnline === 0
          ? "rose"
          : "amber";
  const poolValue = `${poolOnline} / ${poolTotal} ONLINE`;
  const poolDetail = pools
    ? `${pools.length} pool${pools.length !== 1 ? "s" : ""} configured`
    : "loading...";

  const cpuPct = system
    ? Math.round(system.cpu.percent_user + system.cpu.percent_system)
    : 0;
  const sysColor: PipColor = system
    ? cpuPct > 80
      ? "rose"
      : cpuPct > 50
        ? "amber"
        : "emerald"
    : "amber";
  const sysValue = system ? `${cpuPct}% CPU` : "-- CPU";
  const sysDetail = system
    ? `${system.temperature_c.toFixed(0)}\u00B0C \u00B7 ${formatPercent(system.memory.percent_used)} mem`
    : "loading...";

  return (
    <div className="col-span-12 grid grid-cols-4 gap-4">
      <HealthCard
        label="WAN Uplink"
        color={wanColor}
        value={wanValue}
        detail={wanDetail}
      />
      <HealthCard
        label="DNS Resolver"
        color={dnsColor}
        value={dnsValue}
        detail={dnsDetail}
      />
      <HealthCard
        label="VPN Pools"
        color={poolColor}
        value={poolValue}
        detail={poolDetail}
      />
      <HealthCard
        label="System Load"
        color={sysColor}
        value={sysValue}
        detail={sysDetail}
      />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  ThroughputCard                                                    */
/* ------------------------------------------------------------------ */

function ThroughputCard({ traffic, stale, updatedAt }: {
  traffic: Traffic | undefined;
  stale: boolean;
  updatedAt: string | null;
}) {
  const wan = traffic?.interfaces.find((i) => i.role === "wan");
  const rxSamples = wan?.samples_60s.map((s) => s.rx_bps) ?? [];
  const txSamples = wan?.samples_60s.map((s) => s.tx_bps) ?? [];

  return (
    <div className="col-span-8 bg-surface-container rounded-sm p-4">
      <div className="flex items-center justify-between mb-3">
        <p className="label-xs">WAN Throughput</p>
        <StaleIndicator stale={stale} updatedAt={updatedAt} />
      </div>
      <div className="grid grid-cols-2 gap-4 mb-3">
        <div>
          <span className="text-xs text-on-surface-variant">RX</span>
          <MonoText className="ml-2 text-sm">
            {wan ? formatBps(wan.rx_bps) : "--"}
          </MonoText>
        </div>
        <div>
          <span className="text-xs text-on-surface-variant">TX</span>
          <MonoText className="ml-2 text-sm">
            {wan ? formatBps(wan.tx_bps) : "--"}
          </MonoText>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <Sparkline
          data={rxSamples}
          color="hsl(var(--emerald))"
          className="h-16"
        />
        <Sparkline
          data={txSamples}
          color="hsl(var(--info))"
          className="h-16"
        />
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  CriticalAlerts                                                    */
/* ------------------------------------------------------------------ */

type Alert = { level: "rose" | "amber"; message: string };

function CriticalAlerts({
  pools,
  tempC,
  wanOperstate,
}: {
  pools: Pool[] | undefined;
  tempC: number | undefined;
  wanOperstate: string | undefined;
}) {
  const alerts: Alert[] = [];

  if (wanOperstate !== undefined && wanOperstate !== "up") {
    alerts.push({
      level: "rose",
      message: `WAN link ${wanOperstate} — no internet`,
    });
  }

  if (pools) {
    for (const p of pools) {
      if (p.failsafe_drop_active) {
        alerts.push({
          level: "rose",
          message: `Pool "${p.name}" failsafe-drop active`,
        });
      }
      const healthy = p.members.filter((m) => m.healthy).length;
      if (healthy > 0 && healthy < p.members.length) {
        alerts.push({
          level: "amber",
          message: `Pool "${p.name}" degraded (${healthy}/${p.members.length})`,
        });
      }
      if (healthy === 0 && p.members.length > 0) {
        alerts.push({
          level: "rose",
          message: `Pool "${p.name}" all members down`,
        });
      }
    }
  }

  if (tempC !== undefined && tempC > 70) {
    alerts.push({
      level: tempC > 80 ? "rose" : "amber",
      message: `CPU temperature ${tempC.toFixed(0)}\u00B0C`,
    });
  }

  return (
    <div className="col-span-4 bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-3">Critical Alerts</p>
      {alerts.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono">
          No active alerts
        </p>
      ) : (
        <ul className="space-y-2">
          {alerts.map((a, i) => (
            <li key={i} className="flex items-start gap-2 text-sm">
              <span
                className={cn(
                  "mt-1 inline-block h-2 w-2 shrink-0 rounded-full",
                  a.level === "rose" ? "bg-rose" : "bg-amber",
                )}
              />
              <MonoText
                className={cn(
                  "text-xs",
                  a.level === "rose" ? "text-rose" : "text-amber",
                )}
              >
                {a.message}
              </MonoText>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  QuickStats                                                        */
/* ------------------------------------------------------------------ */

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className="text-lg font-semibold">{value}</MonoText>
    </div>
  );
}

function QuickStats({
  system,
  tunnels,
  clients,
  adguard,
}: {
  system: System | undefined;
  tunnels: Tunnel[] | undefined;
  clients: Client[] | undefined;
  adguard: AdguardStats | undefined;
}) {
  const activeClients = clients?.length ?? 0;
  const healthyTunnels = tunnels
    ? tunnels.filter((t) => t.healthy).length
    : 0;
  const totalTunnels = tunnels?.length ?? 0;
  const blockRate = adguard
    ? formatPercent(adguard.block_rate)
    : "--%";
  const queriesToday = adguard
    ? adguard.queries_24h.toLocaleString()
    : "--";
  const uptime = system
    ? formatDuration(system.uptime_seconds)
    : "--";

  return (
    <div className="col-span-12 grid grid-cols-5 gap-4">
      <StatTile label="Active Clients" value={String(activeClients)} />
      <StatTile
        label="Healthy Tunnels"
        value={`${healthyTunnels} / ${totalTunnels}`}
      />
      <StatTile label="AdGuard Block Rate" value={blockRate} />
      <StatTile label="Queries Today" value={queriesToday} />
      <StatTile label="Uptime" value={uptime} />
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Overview Page                                                     */
/* ------------------------------------------------------------------ */

export function Overview() {
  const systemQ = useQuery({
    queryKey: queryKeys.system(),
    queryFn: api.system,
    refetchInterval: 2_000,
  });

  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 2_000,
  });

  const poolsQ = useQuery({
    queryKey: queryKeys.pools(),
    queryFn: api.pools,
    refetchInterval: 5_000,
  });

  const tunnelsQ = useQuery({
    queryKey: queryKeys.tunnels(),
    queryFn: api.tunnels,
    refetchInterval: 5_000,
  });

  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 10_000,
  });

  const adguardQ = useQuery({
    queryKey: queryKeys.adguardStats(),
    queryFn: api.adguardStats,
    refetchInterval: 10_000,
  });

  const system = systemQ.data?.data;
  const traffic = trafficQ.data?.data;
  const pools = poolsQ.data?.data.pools;
  const tunnels = tunnelsQ.data?.data.tunnels;
  const clients = clientsQ.data?.data.clients;
  const adguard = adguardQ.data?.data;

  // Find the WAN interface by backend-provided role so the UI works on
  // deployments with non-default uplink names. Falls through to
  // undefined (rendered as "unknown" / loading) if the backend has not
  // yet populated the field.
  const wanOperstate = traffic?.interfaces.find((i) => i.role === "wan")
    ?.operstate;

  return (
    <div className="grid grid-cols-12 gap-4">
      <HealthStrip
        system={system}
        pools={pools}
        wanOperstate={wanOperstate}
      />

      <ThroughputCard
        traffic={traffic}
        stale={trafficQ.data?.stale ?? false}
        updatedAt={trafficQ.data?.updated_at ?? null}
      />

      <CriticalAlerts
        pools={pools}
        tempC={system?.temperature_c}
        wanOperstate={wanOperstate}
      />

      <QuickStats
        system={system}
        tunnels={tunnels}
        clients={clients}
        adguard={adguard}
      />
    </div>
  );
}
