// Corresponds to Go trace package types

export type SpanKind =
  | "agent_execute"
  | "llm_call"
  | "tool_exec"
  | "sub_agent"
  | "event";

export type SpanStatus = "ok" | "error";

export interface Trace {
  trace_id: string;
  root_span: Span | null;
  metadata: TraceMetadata;
  started_at: string;
  ended_at: string;
}

export interface TraceMetadata {
  model?: string;
  strategy?: string;
  labels?: Record<string, string>;
}

export interface Span {
  span_id: string;
  parent_id?: string;
  kind: SpanKind;
  name: string;
  started_at: string;
  ended_at: string;
  duration: number; // nanoseconds
  status: SpanStatus;
  error?: string;
  children?: Span[];
  llm_call?: LLMCallData;
  tool_exec?: ToolExecData;
  event?: EventData;
}

export interface LLMCallData {
  input_tokens: number;
  output_tokens: number;
  model?: string;
  request?: LLMRequest;
  response?: LLMResponse;
}

export interface LLMRequest {
  system_prompt?: string;
  messages?: Message[];
  tools?: ToolSpec[];
}

export interface LLMResponse {
  texts?: string[];
  function_calls?: FunctionCall[];
}

export interface Message {
  role: string;
  content: string;
}

export interface ToolSpec {
  name: string;
  description?: string;
}

export interface FunctionCall {
  id: string;
  name: string;
  arguments?: Record<string, unknown>;
}

export interface ToolExecData {
  tool_name: string;
  args: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: string;
}

export interface EventData {
  kind: string;
  data: unknown;
}

// API response types
export interface TraceSummary {
  trace_id: string;
  size: number;
  updated_at: string;
}

export interface ListTracesResponse {
  traces: TraceSummary[];
  next_page_token?: string;
}
