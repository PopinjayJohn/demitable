// Display helpers: vault values are often wikilinks like
// "[[The Salt Debt]]" (links are identity). The brackets must never
// reach the UI, and stripping is display-only — writes always send the
// original value.
export function displayName(value: unknown): string {
  const s = String(value ?? "");
  const m = s.match(/^\[\[([^\]|]+?)(?:\|([^\]]+?))?\]\]$/);
  if (m) return (m[2] ?? m[1]).trim();
  return s;
}
