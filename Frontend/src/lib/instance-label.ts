export function formatInstanceLabel(instanceID: string): string {
  const trimmed = String(instanceID || "").trim();
  if (!trimmed) return "i-unknown";
  return `i-${trimmed.slice(0, 6)}`;
}
