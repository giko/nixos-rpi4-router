import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";

export function TunnelBadge({
  name,
  healthy,
  className,
}: {
  name: string;
  healthy: boolean;
  className?: string;
}) {
  return (
    <Link
      to={`/vpn/tunnels/${encodeURIComponent(name)}`}
      className={cn(
        "inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider transition-colors",
        healthy
          ? "bg-info/10 text-info hover:bg-info/20"
          : "bg-rose/10 text-rose hover:bg-rose/20",
        className,
      )}
    >
      {name}
    </Link>
  );
}
