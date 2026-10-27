function stripCalloutMarker(line: string): string {
  let prefixLen = 0;
  while (prefixLen < line.length) {
    const c = line[prefixLen];
    if (c === " " || c === "\t" || c === ">") { prefixLen++; continue; }
    break;
  }
  const t = line.slice(prefixLen);
  if (!t.startsWith("[!")) return line;
  const end = t.indexOf("]");
  if (end < 0) return line;
  let rest = t.slice(end + 1);
  rest = rest.replace(/^\+/, "").replace(/^-/, "");
  return line.slice(0, prefixLen) + rest;
}

export function preprocessSource(src: string): string {
  const lines = src.replace(/\r\n/g, "\n").split("\n");
  const out: string[] = [];
  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith("^") && trimmed.length > 1 && !/[ \t]/.test(trimmed)) continue;
    out.push(stripCalloutMarker(line));
  }
  return out.join("\n");
}
