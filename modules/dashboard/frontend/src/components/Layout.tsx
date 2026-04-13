import { Outlet } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { Sidebar } from "./Sidebar";

// Derives overall system status from traffic (WAN link) and pools
// (tunnel health). These queries are already cached by TanStack Query
// from the Overview page, so no extra network cost.
function HealthPip() {
  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 5_000,
  });
  const poolsQ = useQuery({
    queryKey: queryKeys.pools(),
    queryFn: api.pools,
    refetchInterval: 5_000,
  });

  // Prefer the backend-provided role over a hardcoded interface name so
  // deployments with custom uplinks work. If role is missing (older
  // backend), treat WAN status as unknown rather than reporting "down".
  const wanIf = trafficQ.data?.data?.interfaces?.find(
    (i) => i.role === "wan",
  );
  const wanUp = wanIf?.operstate === "up";
  const wanKnown = wanIf !== undefined;
  const pools = poolsQ.data?.data?.pools ?? [];
  const totalMembers = pools.reduce((n, p) => n + p.members.length, 0);
  const healthyMembers = pools.reduce(
    (n, p) => n + p.members.filter((m) => m.healthy).length,
    0,
  );

  // Determine aggregate status
  let label: string;
  let color: "emerald" | "amber" | "rose";

  if (!trafficQ.data) {
    // Still loading
    label = "Loading";
    color = "amber";
  } else if (!wanKnown) {
    label = "WAN Unknown";
    color = "amber";
  } else if (!wanUp) {
    label = "WAN Down";
    color = "rose";
  } else if (totalMembers > 0 && healthyMembers === 0) {
    label = "Pools Down";
    color = "rose";
  } else if (totalMembers > 0 && healthyMembers < totalMembers) {
    label = "Degraded";
    color = "amber";
  } else {
    label = "Healthy";
    color = "emerald";
  }

  return (
    <span className="inline-flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider">
      <span
        className={cn(
          "inline-block h-1.5 w-1.5 rounded-full",
          color === "emerald" && "bg-emerald",
          color === "amber" && "bg-amber",
          color === "rose" && "bg-rose",
        )}
      />
      <span
        className={cn(
          color === "emerald" && "text-emerald",
          color === "amber" && "text-amber",
          color === "rose" && "text-rose",
        )}
      >
        {label}
      </span>
    </span>
  );
}

export function Layout() {
  return (
    <div className="flex min-h-screen bg-background">
      <Sidebar />
      <div className="flex flex-1 flex-col pl-64">
        <header className="flex h-12 items-center justify-end px-6 bg-surface-low">
          <HealthPip />
        </header>
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
