export function canSubmitReset(phrase: string, resourceName: string, confirmed: boolean, allowed: boolean): boolean {
  return allowed && confirmed && phrase.trim() === resourceName && resourceName !== "";
}

export function canSubmitClear(phrase: string, confirmed: boolean, allowed: boolean): boolean {
  return allowed && confirmed && phrase.trim() === "messages";
}
