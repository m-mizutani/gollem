import { useState, useCallback } from "react";
import type { Span, SpanKind } from "../api/types";
import { formatDuration } from "../utils/format";

interface SpanTreeProps {
  rootSpan: Span | null;
  selectedSpan: Span | null;
  onSelectSpan: (span: Span) => void;
}

const kindColors: Record<SpanKind, string> = {
  agent_execute: "bg-gray-200 text-gray-700",
  llm_call: "bg-blue-100 text-blue-700",
  tool_exec: "bg-green-100 text-green-700",
  sub_agent: "bg-purple-100 text-purple-700",
  event: "bg-orange-100 text-orange-700",
};

interface SpanNodeProps {
  span: Span;
  depth: number;
  selectedSpan: Span | null;
  onSelectSpan: (span: Span) => void;
  defaultExpanded: boolean;
}

function SpanNode({
  span,
  depth,
  selectedSpan,
  onSelectSpan,
  defaultExpanded,
}: SpanNodeProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const hasChildren = span.children && span.children.length > 0;
  const isSelected = selectedSpan?.span_id === span.span_id;

  const handleClick = useCallback(() => {
    onSelectSpan(span);
  }, [span, onSelectSpan]);

  const handleToggle = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      setExpanded(!expanded);
    },
    [expanded]
  );

  return (
    <div>
      <div
        className={`flex items-center gap-2 px-2 py-1.5 cursor-pointer text-sm hover:bg-gray-50 ${
          isSelected ? "bg-blue-50 border-l-2 border-blue-500" : ""
        }`}
        style={{ paddingLeft: `${depth * 20 + 8}px` }}
        onClick={handleClick}
      >
        {hasChildren ? (
          <button
            onClick={handleToggle}
            className="w-4 h-4 flex items-center justify-center text-gray-400 hover:text-gray-600 flex-shrink-0"
          >
            {expanded ? "\u25BE" : "\u25B8"}
          </button>
        ) : (
          <span className="w-4 h-4 flex-shrink-0" />
        )}

        <span
          className={`inline-block px-1.5 py-0.5 rounded text-xs font-medium ${
            kindColors[span.kind] || "bg-gray-100 text-gray-600"
          }`}
        >
          {span.kind.replace("_", " ")}
        </span>

        <span className="truncate flex-1" title={span.name}>
          {span.name}
        </span>

        <span className="text-xs text-gray-400 flex-shrink-0">
          {formatDuration(span.duration)}
        </span>

        {span.status === "error" && (
          <span className="w-2 h-2 rounded-full bg-red-500 flex-shrink-0" />
        )}
      </div>

      {expanded &&
        hasChildren &&
        span.children!.map((child) => (
          <SpanNode
            key={child.span_id}
            span={child}
            depth={depth + 1}
            selectedSpan={selectedSpan}
            onSelectSpan={onSelectSpan}
            defaultExpanded={depth < 1}
          />
        ))}
    </div>
  );
}

export default function SpanTree({
  rootSpan,
  selectedSpan,
  onSelectSpan,
}: SpanTreeProps) {
  if (!rootSpan) {
    return (
      <div className="p-4 text-sm text-gray-500">No span data available</div>
    );
  }

  return (
    <div className="py-1">
      <SpanNode
        span={rootSpan}
        depth={0}
        selectedSpan={selectedSpan}
        onSelectSpan={onSelectSpan}
        defaultExpanded={true}
      />
    </div>
  );
}
