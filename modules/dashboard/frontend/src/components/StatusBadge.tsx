import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

export type StatusKind = "healthy" | "degraded" | "failed" | "info" | "muted";

const KIND_STYLES: Record<StatusKind, string> = {
  healthy: "bg-emerald/10 text-emerald",
  degraded: "bg-amber/10 text-amber",
  failed: "bg-rose/10 text-rose",
  info: "bg-info/10 text-info",
  muted: "bg-surface-high text-on-surface-variant",
};

export function StatusBadge({
  kind = "muted",
  children,
  className,
}: {
  kind?: StatusKind;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center px-2 py-0.5 rounded-sm text-[10px] font-bold uppercase tracking-wider",
        KIND_STYLES[kind],
        className,
      )}
    >
      {children}
    </span>
  );
}
