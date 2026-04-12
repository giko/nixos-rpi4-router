import { cn } from "@/lib/utils";
import { formatRelativeAgo } from "@/lib/formatters";

export function StaleIndicator({
  stale,
  updatedAt,
  className,
}: {
  stale: boolean;
  updatedAt: string | null;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 text-[10px] font-bold uppercase tracking-wider",
        stale ? "text-amber" : "text-emerald",
        className,
      )}
    >
      <span
        className={cn(
          "inline-block h-1.5 w-1.5 rounded-full",
          stale ? "bg-amber" : "bg-emerald",
        )}
      />
      {stale ? `Stale \u00B7 ${formatRelativeAgo(updatedAt)}` : "Live"}
    </span>
  );
}
