import { Outlet } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { Sidebar } from "./Sidebar";

function HealthPip() {
  const { data, isError } = useQuery({
    queryKey: queryKeys.health(),
    queryFn: api.health,
    refetchInterval: 10_000,
  });

  const ok = data?.ok === true && !isError;
  return (
    <span className="inline-flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider">
      <span
        className={cn(
          "inline-block h-1.5 w-1.5 rounded-full",
          ok ? "bg-emerald" : "bg-rose",
        )}
      />
      <span className={ok ? "text-emerald" : "text-rose"}>
        {ok ? "Healthy" : "Unhealthy"}
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
