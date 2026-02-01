import { useState, useMemo, useCallback, useRef } from "react";
import type { Trace, Span, SpanKind } from "../api/types";
import { formatDuration } from "../utils/format";

interface TimelineProps {
  trace: Trace;
}

const SPAN_COLORS: Record<SpanKind, string> = {
  agent_execute: "#94a3b8",
  llm_call: "#3b82f6",
  tool_exec: "#22c55e",
  sub_agent: "#a855f7",
  event: "#f97316",
};

const ROW_HEIGHT = 28;
const LABEL_WIDTH = 200;
const TOP_MARGIN = 40;
const LEFT_PADDING = 10;

interface FlatSpan {
  span: Span;
  depth: number;
  index: number;
}

function flattenSpans(span: Span, depth: number, result: FlatSpan[]): void {
  result.push({ span, depth, index: result.length });
  for (const child of span.children || []) {
    flattenSpans(child, depth + 1, result);
  }
}

export default function Timeline({ trace }: TimelineProps) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [hoveredSpan, setHoveredSpan] = useState<FlatSpan | null>(null);
  const [tooltipPos, setTooltipPos] = useState({ x: 0, y: 0 });

  const flatSpans = useMemo(() => {
    if (!trace.root_span) return [];
    const result: FlatSpan[] = [];
    flattenSpans(trace.root_span, 0, result);
    return result;
  }, [trace]);

  const traceStartMs = useMemo(
    () => new Date(trace.started_at).getTime(),
    [trace]
  );
  const traceEndMs = useMemo(
    () => new Date(trace.ended_at).getTime(),
    [trace]
  );
  const traceDurationMs = traceEndMs - traceStartMs;

  const chartWidth = 800;
  const chartHeight = flatSpans.length * ROW_HEIGHT + TOP_MARGIN + 20;
  const barAreaWidth = chartWidth - LABEL_WIDTH - LEFT_PADDING;

  const msToX = useCallback(
    (ms: number) => {
      if (traceDurationMs === 0) return 0;
      return LABEL_WIDTH + LEFT_PADDING + (ms / traceDurationMs) * barAreaWidth;
    },
    [traceDurationMs, barAreaWidth]
  );

  const scaleMarks = useMemo(() => {
    const marks: { ms: number; label: string }[] = [];
    if (traceDurationMs === 0) return marks;
    const step = Math.pow(10, Math.floor(Math.log10(traceDurationMs)));
    const nice =
      traceDurationMs / step < 3
        ? step / 2
        : traceDurationMs / step < 6
        ? step
        : step * 2;
    for (let ms = 0; ms <= traceDurationMs; ms += nice) {
      marks.push({
        ms,
        label: formatDuration(ms * 1_000_000),
      });
    }
    return marks;
  }, [traceDurationMs]);

  const handleMouseMove = useCallback(
    (e: React.MouseEvent<SVGElement>, fs: FlatSpan) => {
      if (!svgRef.current) return;
      const svgRect = svgRef.current.getBoundingClientRect();
      setTooltipPos({ x: e.clientX - svgRect.left + 10, y: e.clientY - svgRect.top - 10 });
      setHoveredSpan(fs);
    },
    []
  );

  if (flatSpans.length === 0) {
    return (
      <div className="bg-white border border-gray-200 rounded-lg p-8 text-center text-gray-500">
        No span data available
      </div>
    );
  }

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4 overflow-x-auto">
      <svg
        ref={svgRef}
        width={chartWidth}
        height={chartHeight}
        className="font-mono text-xs"
      >
        {/* Scale bar */}
        {scaleMarks.map((mark, i) => {
          const x = msToX(mark.ms);
          return (
            <g key={i}>
              <line
                x1={x}
                y1={TOP_MARGIN - 5}
                x2={x}
                y2={chartHeight}
                stroke="#e5e7eb"
                strokeWidth={1}
              />
              <text
                x={x}
                y={TOP_MARGIN - 12}
                textAnchor="middle"
                fill="#9ca3af"
                fontSize={10}
              >
                {mark.label}
              </text>
            </g>
          );
        })}

        {/* Span bars */}
        {flatSpans.map((fs) => {
          const spanStartMs =
            new Date(fs.span.started_at).getTime() - traceStartMs;
          const spanDurationMs = fs.span.duration / 1_000_000;
          const x = msToX(spanStartMs);
          const w = Math.max((spanDurationMs / traceDurationMs) * barAreaWidth, 2);
          const y = TOP_MARGIN + fs.index * ROW_HEIGHT;
          const color = SPAN_COLORS[fs.span.kind] || "#94a3b8";
          const isEvent = fs.span.kind === "event";
          const isError = fs.span.status === "error";

          return (
            <g
              key={fs.span.span_id}
              onMouseMove={(e) => handleMouseMove(e, fs)}
              onMouseLeave={() => setHoveredSpan(null)}
              className="cursor-pointer"
            >
              {/* Label */}
              <text
                x={fs.depth * 12 + 4}
                y={y + ROW_HEIGHT / 2 + 4}
                fill="#374151"
                fontSize={11}
                className="truncate"
              >
                {fs.span.name.length > 20
                  ? fs.span.name.slice(0, 18) + ".."
                  : fs.span.name}
              </text>

              {/* Bar or marker */}
              {isEvent ? (
                <circle
                  cx={x}
                  cy={y + ROW_HEIGHT / 2}
                  r={4}
                  fill={color}
                  stroke={isError ? "#ef4444" : "none"}
                  strokeWidth={isError ? 2 : 0}
                />
              ) : (
                <rect
                  x={x}
                  y={y + 4}
                  width={w}
                  height={ROW_HEIGHT - 8}
                  rx={3}
                  fill={color}
                  opacity={0.8}
                  stroke={isError ? "#ef4444" : "none"}
                  strokeWidth={isError ? 2 : 0}
                />
              )}
            </g>
          );
        })}

        {/* Tooltip */}
        {hoveredSpan && (
          <foreignObject
            x={tooltipPos.x}
            y={tooltipPos.y}
            width={250}
            height={80}
          >
            <div className="bg-gray-900 text-white text-xs rounded px-3 py-2 shadow-lg">
              <div className="font-medium">{hoveredSpan.span.name}</div>
              <div className="text-gray-300 mt-1">
                Kind: {hoveredSpan.span.kind} | Duration:{" "}
                {formatDuration(hoveredSpan.span.duration)} | Status:{" "}
                {hoveredSpan.span.status}
              </div>
            </div>
          </foreignObject>
        )}
      </svg>
    </div>
  );
}
