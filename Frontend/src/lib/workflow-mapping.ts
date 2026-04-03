export function parseFieldMapping(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  for (const pair of raw.split(",")) {
    const [source, target] = pair.split(":").map((v) => v.trim());
    if (source && target) out[source] = target;
  }
  return out;
}

export function serializeFieldMapping(mapping: Record<string, string>): string {
  return Object.entries(mapping)
    .filter(([source, target]) => source.trim() && target.trim())
    .map(([source, target]) => `${source}:${target}`)
    .join(", ");
}
