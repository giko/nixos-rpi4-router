import { useQuery } from "@tanstack/react-query";

type HealthResponse = {
  ok: boolean;
  version: string;
  started_at: string;
};

async function fetchHealth(): Promise<HealthResponse> {
  const res = await fetch("/api/health");
  if (!res.ok) {
    throw new Error(`health endpoint returned ${res.status}`);
  }
  return res.json();
}

export function App() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["health"],
    queryFn: fetchHealth,
    refetchInterval: 5_000,
  });

  return (
    <main className="min-h-screen flex items-center justify-center p-8">
      <div className="bg-card border border-border/20 rounded-md p-8 min-w-80">
        <h1 className="text-xs tracking-widest uppercase text-muted-foreground mb-2">
          SENTINEL OS
        </h1>
        <p className="text-2xl font-semibold mb-4">Router Dashboard</p>
        {isLoading && (
          <p className="text-muted-foreground font-mono text-sm">…loading health…</p>
        )}
        {error && (
          <p className="text-destructive font-mono text-sm">
            health: {(error as Error).message}
          </p>
        )}
        {data && (
          <div className="font-mono text-sm space-y-1">
            <p>
              <span className="text-muted-foreground">status: </span>
              <span className="text-primary">{data.ok ? "OK" : "FAIL"}</span>
            </p>
            <p>
              <span className="text-muted-foreground">version: </span>
              {data.version}
            </p>
            <p>
              <span className="text-muted-foreground">started: </span>
              {new Date(data.started_at).toLocaleString()}
            </p>
          </div>
        )}
      </div>
    </main>
  );
}
