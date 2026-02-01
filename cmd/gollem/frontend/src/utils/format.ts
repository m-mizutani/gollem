// Format nanoseconds duration to human-readable string
export function formatDuration(ns: number): string {
  if (ns < 1_000_000) {
    return `${(ns / 1_000).toFixed(1)}µs`;
  }
  if (ns < 1_000_000_000) {
    return `${(ns / 1_000_000).toFixed(1)}ms`;
  }
  const seconds = ns / 1_000_000_000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }
  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  return `${minutes}m ${remainingSeconds.toFixed(0)}s`;
}

// Format bytes to human-readable size
export function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// Format ISO date string to relative time
export function formatRelativeTime(isoStr: string): string {
  const date = new Date(isoStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);

  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}

// Format ISO date string to local datetime
export function formatDateTime(isoStr: string): string {
  return new Date(isoStr).toLocaleString();
}

// Compute duration between two ISO date strings in nanoseconds
export function computeDurationNs(
  startedAt: string,
  endedAt: string
): number {
  const start = new Date(startedAt).getTime();
  const end = new Date(endedAt).getTime();
  return (end - start) * 1_000_000; // ms to ns
}

// Pretty-print JSON with indentation
export function prettyJSON(value: unknown): string {
  try {
    if (typeof value === "string") {
      const parsed = JSON.parse(value);
      return JSON.stringify(parsed, null, 2);
    }
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

// Check if a string looks like JSON
export function isJSON(str: string): boolean {
  try {
    JSON.parse(str);
    return true;
  } catch {
    return false;
  }
}

