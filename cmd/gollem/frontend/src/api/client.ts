import type { Entry, ListEntriesResponse, Trace } from "./types";

const BASE_URL = "/api";

async function fetchJSON<T>(path: string): Promise<T> {
  const resp = await fetch(`${BASE_URL}${path}`);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

// encodeTracePath encodes each path segment so that "/" stays as a separator.
function encodeTracePath(path: string): string {
  return path
    .split("/")
    .filter((seg) => seg.length > 0)
    .map((seg) => encodeURIComponent(seg))
    .join("/");
}

export async function listEntries(
  path: string = "",
  pageSize: number = 20,
  pageToken: string = ""
): Promise<ListEntriesResponse> {
  const params = new URLSearchParams();
  params.set("page_size", String(pageSize));
  if (path) {
    params.set("path", path);
  }
  if (pageToken) {
    params.set("page_token", pageToken);
  }
  return fetchJSON<ListEntriesResponse>(`/traces?${params.toString()}`);
}

export async function listAllEntries(path: string = ""): Promise<Entry[]> {
  const all: Entry[] = [];
  let pageToken = "";
  const pageSize = 1000;

  do {
    const resp = await listEntries(path, pageSize, pageToken);
    all.push(...resp.entries);
    pageToken = resp.next_page_token || "";
  } while (pageToken);

  return all;
}

export async function getTrace(tracePath: string): Promise<Trace> {
  return fetchJSON<Trace>(`/traces/${encodeTracePath(tracePath)}`);
}

export async function healthCheck(): Promise<{ status: string }> {
  return fetchJSON<{ status: string }>("/health");
}
