import { useMemo } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import type { Trace, Span } from "../api/types";

interface TokenChartProps {
  trace: Trace;
}

interface LLMCallEntry {
  seq: number;
  name: string;
  model: string;
  inputTokens: number;
  outputTokens: number;
}

function collectLLMCalls(span: Span, result: LLMCallEntry[]): void {
  if (span.kind === "llm_call" && span.llm_call) {
    result.push({
      seq: result.length + 1,
      name: span.name,
      model: span.llm_call.model || "",
      inputTokens: span.llm_call.input_tokens,
      outputTokens: span.llm_call.output_tokens,
    });
  }
  for (const child of span.children || []) {
    collectLLMCalls(child, result);
  }
}

export default function TokenChart({ trace }: TokenChartProps) {
  const data = useMemo(() => {
    if (!trace.root_span) return [];
    const calls: LLMCallEntry[] = [];
    collectLLMCalls(trace.root_span, calls);
    return calls;
  }, [trace]);

  const totals = useMemo(() => {
    return data.reduce(
      (acc, d) => ({
        input: acc.input + d.inputTokens,
        output: acc.output + d.outputTokens,
      }),
      { input: 0, output: 0 }
    );
  }, [data]);

  if (data.length === 0) {
    return (
      <div className="bg-white border border-gray-200 rounded-lg p-6 text-center text-gray-500 text-sm">
        No LLM calls in this trace
      </div>
    );
  }

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <h3 className="font-semibold text-gray-700 mb-4">Token Usage</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data}>
          <XAxis dataKey="seq" label={{ value: "LLM Call #", position: "insideBottom", offset: -5 }} />
          <YAxis label={{ value: "Tokens", angle: -90, position: "insideLeft" }} />
          <Tooltip
            content={({ payload }) => {
              if (!payload || payload.length === 0) return null;
              const entry = payload[0]?.payload as LLMCallEntry;
              return (
                <div className="bg-gray-900 text-white text-xs rounded px-3 py-2 shadow-lg">
                  <div className="font-medium">{entry.name}</div>
                  {entry.model && (
                    <div className="text-gray-300">Model: {entry.model}</div>
                  )}
                  <div className="text-blue-300">
                    Input: {entry.inputTokens}
                  </div>
                  <div className="text-orange-300">
                    Output: {entry.outputTokens}
                  </div>
                </div>
              );
            }}
          />
          <Legend />
          <Bar dataKey="inputTokens" name="Input Tokens" fill="#3b82f6" />
          <Bar dataKey="outputTokens" name="Output Tokens" fill="#f97316" />
        </BarChart>
      </ResponsiveContainer>
      <div className="text-sm text-gray-500 text-center mt-2">
        Total: {totals.input + totals.output} tokens ({totals.input} input /{" "}
        {totals.output} output)
      </div>
    </div>
  );
}
