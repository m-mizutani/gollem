import type { ListTracesResponse, Trace, TraceSummary } from "./types";

const BASE_URL = "/api";

async function fetchJSON<T>(path: string): Promise<T> {
  const resp = await fetch(`${BASE_URL}${path}`);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ error: resp.statusText }));
    throw new Error(body.error || `HTTP ${resp.status}`);
  }
  return resp.json();
}

export async function listTraces(
  pageSize: number = 20,
  pageToken: string = ""
): Promise<ListTracesResponse> {
  const params = new URLSearchParams();
  params.set("page_size", String(pageSize));
  if (pageToken) {
    params.set("page_token", pageToken);
  }
  return fetchJSON<ListTracesResponse>(`/traces?${params.toString()}`);
}

export async function listAllTraces(): Promise<TraceSummary[]> {
  const all: TraceSummary[] = [];
  let pageToken = "";
  const pageSize = 1000;

  do {
    const resp = await listTraces(pageSize, pageToken);
    all.push(...resp.traces);
    pageToken = resp.next_page_token || "";
  } while (pageToken);

  return all;
}

export async function getTrace(traceID: string): Promise<Trace> {
  return fetchJSON<Trace>(`/traces/${encodeURIComponent(traceID)}`);
}

export async function healthCheck(): Promise<{ status: string }> {
  return fetchJSON<{ status: string }>("/health");
}
