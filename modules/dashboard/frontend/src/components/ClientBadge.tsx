import { Link } from "react-router-dom";
import { cn } from "@/lib/utils";
import { MonoText } from "./MonoText";

export function ClientBadge({
  ip,
  label,
  className,
}: {
  ip: string;
  label?: string;
  className?: string;
}) {
  return (
    <Link
      to={`/clients/${encodeURIComponent(ip)}`}
      className={cn(
        "text-sm text-on-surface hover:text-primary transition-colors",
        className,
      )}
    >
      {label || <MonoText>{ip}</MonoText>}
    </Link>
  );
}
