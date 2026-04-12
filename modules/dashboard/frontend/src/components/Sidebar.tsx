import { NavLink } from "react-router-dom";
import { cn } from "@/lib/utils";
import {
  LayoutDashboard,
  Layers,
  Cable,
  Users,
  Shield,
  Activity,
  Flame,
  Cpu,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

type NavItem = { to: string; label: string; icon: LucideIcon };

const NAV_ITEMS: NavItem[] = [
  { to: "/", label: "Dashboard", icon: LayoutDashboard },
  { to: "/vpn/pools", label: "VPN Pools", icon: Layers },
  { to: "/vpn/tunnels", label: "VPN Tunnels", icon: Cable },
  { to: "/clients", label: "Clients", icon: Users },
  { to: "/adguard", label: "AdGuard DNS", icon: Shield },
  { to: "/traffic", label: "Traffic", icon: Activity },
  { to: "/firewall", label: "Firewall & PBR", icon: Flame },
  { to: "/system", label: "System", icon: Cpu },
];

export function Sidebar() {
  return (
    <aside className="fixed inset-y-0 left-0 z-30 flex w-64 flex-col bg-surface-low">
      <div className="px-5 py-6">
        <span className="text-[11px] font-bold uppercase tracking-widest text-on-surface-variant">
          Sentinel OS
        </span>
      </div>
      <nav className="flex-1 space-y-0.5 px-2">
        {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-3 rounded-sm px-3 py-2 text-[0.6875rem] font-bold uppercase tracking-widest transition-colors",
                isActive
                  ? "border-r-2 border-primary bg-surface-high text-primary"
                  : "text-on-surface-variant hover:bg-surface-high hover:text-on-surface",
              )
            }
          >
            <Icon size={16} strokeWidth={1.5} />
            {label}
          </NavLink>
        ))}
      </nav>
    </aside>
  );
}
