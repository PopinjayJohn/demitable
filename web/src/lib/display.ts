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

// Reads a campaign note's `home:` link from raw file content. Only
// callers who already hold the raw file (the GM view) can use this:
// projected bodies have no frontmatter. Returns the link target, or
// null when the field is absent or malformed.
export function parseHomeLink(content: string): string | null {
  const src = String(content ?? "");
  const lines = src.split("\n");
  for (const line of lines) {
    const t = line.trim();
    if (!t.startsWith("home:")) continue;
    const rest = t.slice("home:".length).trim().replace(/^"|"$/g, "");
    if (!rest.startsWith("[[") || !rest.endsWith("]]")) return null;
    const inner = rest.slice(2, -2);
    const target = inner.split("|")[0].trim();
    return target === "" ? null : target;
  }
  return null;
}
