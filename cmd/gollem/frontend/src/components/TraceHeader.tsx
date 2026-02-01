import { Link } from "react-router-dom";
import type { Trace, Span } from "../api/types";
import { formatDuration, computeDurationNs } from "../utils/format";

interface TraceHeaderProps {
  trace: Trace;
}

function collectTokens(span: Span): { input: number; output: number } {
  let input = 0;
  let output = 0;
  if (span.llm_call) {
    input += span.llm_call.input_tokens;
    output += span.llm_call.output_tokens;
  }
  for (const child of span.children || []) {
    const childTokens = collectTokens(child);
    input += childTokens.input;
    output += childTokens.output;
  }
  return { input, output };
}

export default function TraceHeader({ trace }: TraceHeaderProps) {
  const duration = computeDurationNs(trace.started_at, trace.ended_at);
  const status = trace.root_span?.status || "ok";
  const errorMsg = trace.root_span?.error;
  const tokens = trace.root_span ? collectTokens(trace.root_span) : { input: 0, output: 0 };

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-mono font-semibold">{trace.trace_id}</h1>
          <button
            onClick={() => navigator.clipboard.writeText(trace.trace_id)}
            className="text-xs text-gray-400 hover:text-gray-600"
            title="Copy Trace ID"
          >
            Copy
          </button>
        </div>
        <Link
          to="/"
          className="text-sm text-blue-600 hover:text-blue-800"
        >
          Back to list
        </Link>
      </div>

      <div className="flex flex-wrap gap-4 text-sm">
        {trace.metadata.model && (
          <div>
            <span className="text-gray-500">Model:</span>{" "}
            <span className="font-medium">{trace.metadata.model}</span>
          </div>
        )}
        {trace.metadata.strategy && (
          <div>
            <span className="text-gray-500">Strategy:</span>{" "}
            <span className="font-medium">{trace.metadata.strategy}</span>
          </div>
        )}
        <div>
          <span className="text-gray-500">Duration:</span>{" "}
          <span className="font-medium">{formatDuration(duration)}</span>
        </div>
        <div>
          <span className="text-gray-500">Status:</span>{" "}
          <span
            className={`inline-block px-2 py-0.5 rounded text-xs font-medium ${
              status === "ok"
                ? "bg-green-100 text-green-800"
                : "bg-red-100 text-red-800"
            }`}
          >
            {status}
          </span>
        </div>
        <div>
          <span className="text-gray-500">Tokens:</span>{" "}
          <span className="font-medium">
            {tokens.input + tokens.output} ({tokens.input} in / {tokens.output}{" "}
            out)
          </span>
        </div>
      </div>

      {trace.metadata.labels && Object.keys(trace.metadata.labels).length > 0 && (
        <div className="flex gap-2">
          {Object.entries(trace.metadata.labels).map(([key, val]) => (
            <span
              key={key}
              className="inline-block px-2 py-0.5 bg-gray-100 rounded text-xs text-gray-700"
            >
              {key}={val}
            </span>
          ))}
        </div>
      )}

      {errorMsg && (
        <div className="bg-red-50 border border-red-200 rounded p-3 text-sm text-red-700">
          {errorMsg}
        </div>
      )}
    </div>
  );
}
