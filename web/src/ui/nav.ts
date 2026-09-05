export type NavItem = { to: string; label: string };

export function navItems(opts: { canAudit: boolean; canReset: boolean }): NavItem[] {
  const items: NavItem[] = [
    { to: "/", label: "Messages" },
    { to: "/status", label: "Status" },
    { to: "/filters", label: "Filters" },
  ];
  if (opts.canAudit) {
    items.push({ to: "/audit", label: "Audit" });
  }
  if (opts.canReset) {
    items.push({ to: "/reset", label: "Reset" });
  }
  return items;
}
