import { useMemo } from "react";
import {
  PieChart,
  Pie,
  Cell,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from "recharts";
import type { Trace, Span, SpanKind } from "../api/types";
import { formatDuration } from "../utils/format";

interface DurationChartProps {
  trace: Trace;
}

const KIND_COLORS: Record<string, string> = {
  llm_call: "#3b82f6",
  tool_exec: "#22c55e",
  sub_agent: "#a855f7",
  event: "#f97316",
  other: "#94a3b8",
};

interface DurationEntry {
  name: string;
  duration: number;
  color: string;
}

function collectDurations(
  span: Span,
  acc: Record<string, number>
): void {
  const kind = span.kind as string;
  if (!acc[kind]) {
    acc[kind] = 0;
  }
  acc[kind] += span.duration;

  for (const child of span.children || []) {
    collectDurations(child, acc);
  }
}

export default function DurationChart({ trace }: DurationChartProps) {
  const data = useMemo(() => {
    if (!trace.root_span) return [];
    const acc: Record<string, number> = {};
    collectDurations(trace.root_span, acc);

    const entries: DurationEntry[] = Object.entries(acc)
      .filter(([, dur]) => dur > 0)
      .map(([kind, dur]) => ({
        name: kind.replace("_", " "),
        duration: dur,
        color: KIND_COLORS[kind as SpanKind] || KIND_COLORS.other,
      }))
      .sort((a, b) => b.duration - a.duration);

    return entries;
  }, [trace]);

  const totalDuration = useMemo(
    () => data.reduce((sum, d) => sum + d.duration, 0),
    [data]
  );

  if (data.length === 0) {
    return (
      <div className="bg-white border border-gray-200 rounded-lg p-6 text-center text-gray-500 text-sm">
        No duration data available
      </div>
    );
  }

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <h3 className="font-semibold text-gray-700 mb-4">Duration Breakdown</h3>
      <ResponsiveContainer width="100%" height={300}>
        <PieChart>
          <Pie
            data={data}
            dataKey="duration"
            nameKey="name"
            cx="50%"
            cy="50%"
            outerRadius={100}
            label={({ name, percent }) =>
              `${name} (${(percent * 100).toFixed(0)}%)`
            }
          >
            {data.map((entry, i) => (
              <Cell key={i} fill={entry.color} />
            ))}
          </Pie>
          <Tooltip
            content={({ payload }) => {
              if (!payload || payload.length === 0) return null;
              const entry = payload[0]?.payload as DurationEntry;
              const pct = totalDuration > 0 ? ((entry.duration / totalDuration) * 100).toFixed(1) : "0";
              return (
                <div className="bg-gray-900 text-white text-xs rounded px-3 py-2 shadow-lg">
                  <div className="font-medium">{entry.name}</div>
                  <div>{formatDuration(entry.duration)}</div>
                  <div>{pct}%</div>
                </div>
              );
            }}
          />
          <Legend />
        </PieChart>
      </ResponsiveContainer>
    </div>
  );
}
