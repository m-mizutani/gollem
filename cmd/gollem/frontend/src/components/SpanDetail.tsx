import { useState } from "react";
import type { Span } from "../api/types";
import { formatDuration, formatDateTime } from "../utils/format";
import LLMCallDetail from "./LLMCallDetail";
import ToolExecDetail from "./ToolExecDetail";
import EventDetail from "./EventDetail";

interface SpanDetailProps {
  span: Span | null;
}

function StackTraceSection({ frames }: { frames: Span["stack_trace"] }) {
  const [expanded, setExpanded] = useState(false);
  if (!frames || frames.length === 0) return null;

  // Extract short file name from full path
  const shortFile = (file: string) => {
    const parts = file.split("/");
    return parts.length > 2
      ? parts.slice(-2).join("/")
      : file;
  };

  return (
    <div className="border border-gray-200 rounded">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-2 px-3 py-2 text-xs text-gray-500 hover:bg-gray-50"
      >
        <span className="flex-shrink-0">{expanded ? "\u25BE" : "\u25B8"}</span>
        <span>Stack Trace ({frames.length} frames)</span>
      </button>
      {expanded && (
        <div className="border-t border-gray-200 bg-gray-50 px-3 py-2 overflow-x-auto">
          <table className="text-xs font-mono w-full">
            <tbody>
              {frames.map((frame, i) => (
                <tr key={i} className={i === 0 ? "text-gray-900 font-medium" : "text-gray-500"}>
                  <td className="pr-2 py-0.5 text-right text-gray-400 select-none w-6">{i}</td>
                  <td className="pr-3 py-0.5 whitespace-nowrap">{frame.function.split("/").pop()}</td>
                  <td className="py-0.5 whitespace-nowrap text-gray-400">
                    {shortFile(frame.file)}:{frame.line}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
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

      {span.stack_trace && span.stack_trace.length > 0 && (
        <StackTraceSection frames={span.stack_trace} />
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
