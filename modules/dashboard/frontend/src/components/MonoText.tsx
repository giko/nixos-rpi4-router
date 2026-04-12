import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

export function MonoText({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <span className={cn("font-mono tabular-nums", className)}>{children}</span>
  );
}
