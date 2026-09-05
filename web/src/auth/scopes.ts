export const SCOPE_READ = "syslog.read";
export const SCOPE_WRITE = "syslog.write";
export const SCOPE_ADMIN = "syslog.admin";
export const SCOPE_AUDIT = "syslog.audit.read";

export function hasScope(scopes: readonly string[], need: string): boolean {
  return scopes.includes(need);
}
