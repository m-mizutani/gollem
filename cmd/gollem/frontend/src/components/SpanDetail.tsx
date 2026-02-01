import type { Span } from "../api/types";
import { formatDuration, formatDateTime } from "../utils/format";
import LLMCallDetail from "./LLMCallDetail";
import ToolExecDetail from "./ToolExecDetail";
import EventDetail from "./EventDetail";

interface SpanDetailProps {
  span: Span | null;
}

export default function SpanDetail({ span }: SpanDetailProps) {
  if (!span) {
    return (
      <div className="p-6 text-sm text-gray-500 flex items-center justify-center h-full">
        Select a span from the tree to view details
      </div>
    );
  }

  return (
    <div className="p-4 space-y-4 text-sm overflow-auto">
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <h3 className="font-semibold text-base">{span.name}</h3>
          <button
            onClick={() => navigator.clipboard.writeText(span.span_id)}
            className="text-xs text-gray-400 hover:text-gray-600"
            title="Copy Span ID"
          >
            [{span.span_id.slice(0, 8)}...]
          </button>
        </div>

        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
          <div>
            <span className="text-gray-500">Kind:</span>{" "}
            <span className="inline-block px-1.5 py-0.5 rounded text-xs font-medium bg-gray-100">
              {span.kind}
            </span>
          </div>
          <div>
            <span className="text-gray-500">Status:</span>{" "}
            <span
              className={`inline-block px-1.5 py-0.5 rounded text-xs font-medium ${
                span.status === "ok"
                  ? "bg-green-100 text-green-700"
                  : "bg-red-100 text-red-700"
              }`}
            >
              {span.status}
            </span>
          </div>
          <div>
            <span className="text-gray-500">Started:</span>{" "}
            {formatDateTime(span.started_at)}
          </div>
          <div>
            <span className="text-gray-500">Ended:</span>{" "}
            {formatDateTime(span.ended_at)}
          </div>
          <div>
            <span className="text-gray-500">Duration:</span>{" "}
            {formatDuration(span.duration)}
          </div>
        </div>
      </div>

      {span.error && (
        <div className="bg-red-50 border border-red-200 rounded p-3 text-red-700">
          <span className="font-medium">Error:</span> {span.error}
        </div>
      )}

      {span.kind === "llm_call" && span.llm_call && (
        <LLMCallDetail data={span.llm_call} />
      )}

      {span.kind === "tool_exec" && span.tool_exec && (
        <ToolExecDetail data={span.tool_exec} />
      )}

      {span.kind === "event" && span.event && (
        <EventDetail data={span.event} />
      )}
    </div>
  );
}
