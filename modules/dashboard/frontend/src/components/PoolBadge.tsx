import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";

export function PoolBadge({
  name,
  className,
}: {
  name: string;
  className?: string;
}) {
  return (
    <Link
      to={`/vpn/pools/${encodeURIComponent(name)}`}
      className={cn(
        "inline-flex items-center rounded-sm px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider bg-primary/10 text-primary hover:bg-primary/20 transition-colors",
        className,
      )}
    >
      {name}
    </Link>
  );
}
