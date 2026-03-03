import { useState, useMemo } from "react";
import type { Trace, Span } from "../api/types";
import { formatDuration } from "../utils/format";
import LLMCallDetail from "./LLMCallDetail";

interface LLMCallListProps {
  trace: Trace;
}

interface LLMCallEntry {
  span: Span;
  seq: number;
  parentName: string;
}

function collectLLMCalls(
  span: Span,
  result: LLMCallEntry[],
  parentName: string
): void {
  if (span.kind === "llm_call" && span.llm_call) {
    result.push({ span, seq: result.length + 1, parentName });
  }
  const nextParent =
    span.kind === "agent_execute" || span.kind === "sub_agent"
      ? span.name
      : parentName;
  for (const child of span.children || []) {
    collectLLMCalls(child, result, nextParent);
  }
}

export default function LLMCallList({ trace }: LLMCallListProps) {
  const [expandedSeq, setExpandedSeq] = useState<number | null>(null);

  const calls = useMemo(() => {
    if (!trace.root_span) return [];
    const result: LLMCallEntry[] = [];
    collectLLMCalls(trace.root_span, result, "root");
    return result;
  }, [trace]);

  const totals = useMemo(() => {
    return calls.reduce(
      (acc, c) => ({
        input: acc.input + (c.span.llm_call?.input_tokens ?? 0),
        output: acc.output + (c.span.llm_call?.output_tokens ?? 0),
        duration: acc.duration + c.span.duration,
      }),
      { input: 0, output: 0, duration: 0 }
    );
  }, [calls]);

  if (calls.length === 0) {
    return (
      <div className="bg-white border border-gray-200 rounded-lg p-8 text-center text-gray-500 text-sm">
        No LLM calls in this trace
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Summary */}
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <div className="flex gap-6 text-sm">
          <div>
            <span className="text-gray-500">LLM Calls:</span>{" "}
            <span className="font-medium">{calls.length}</span>
          </div>
          <div>
            <span className="text-gray-500">Total Tokens:</span>{" "}
            <span className="font-medium">
              {totals.input + totals.output} ({totals.input} in /{" "}
              {totals.output} out)
            </span>
          </div>
          <div>
            <span className="text-gray-500">Total LLM Duration:</span>{" "}
            <span className="font-medium">
              {formatDuration(totals.duration)}
            </span>
          </div>
        </div>
      </div>

      {/* Call list */}
      <div className="space-y-2">
        {calls.map((entry) => {
          const llm = entry.span.llm_call!;
          const isExpanded = expandedSeq === entry.seq;
          const isError = entry.span.status === "error";

          return (
            <div
              key={entry.seq}
              className={`bg-white border rounded-lg ${
                isError ? "border-red-300" : "border-gray-200"
              }`}
            >
              <button
                onClick={() =>
                  setExpandedSeq(isExpanded ? null : entry.seq)
                }
                className="w-full px-4 py-3 text-left hover:bg-gray-50 flex items-center gap-4"
              >
                <span className="text-xs text-gray-400 font-mono w-6">
                  #{entry.seq}
                </span>
                <span className="text-sm font-medium flex-1 truncate">
                  {entry.span.name}
                </span>
                {llm.model && (
                  <span className="text-xs text-gray-500 bg-gray-100 px-2 py-0.5 rounded">
                    {llm.model}
                  </span>
                )}
                <span className="text-xs text-gray-500">
                  {llm.input_tokens + llm.output_tokens} tokens
                </span>
                <span className="text-xs text-gray-500">
                  {formatDuration(entry.span.duration)}
                </span>
                {isError && (
                  <span className="text-xs text-red-600 font-medium">
                    error
                  </span>
                )}
                <span className="text-gray-400 text-xs">
                  {isExpanded ? "▾" : "▸"}
                </span>
              </button>
              {isExpanded && (
                <div className="px-4 pb-4 border-t border-gray-100">
                  <div className="pt-3">
                    <LLMCallDetail data={llm} />
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
