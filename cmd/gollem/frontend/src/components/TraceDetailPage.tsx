import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { useTrace } from "../hooks/useTrace";
import type { Span } from "../api/types";
import TraceHeader from "./TraceHeader";
import SpanTree from "./SpanTree";
import SpanDetail from "./SpanDetail";
import Timeline from "./Timeline";
import TokenChart from "./TokenChart";
import DurationChart from "./DurationChart";

type Tab = "overview" | "timeline" | "charts";

export default function TraceDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: trace, isLoading, error } = useTrace(id || "");
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [selectedSpan, setSelectedSpan] = useState<Span | null>(null);

  if (isLoading) {
    return (
      <div className="max-w-7xl mx-auto p-8 text-center text-gray-500">
        Loading trace...
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto">
        <div className="bg-red-50 border border-red-200 rounded-lg p-6">
          <h2 className="text-lg font-medium text-red-800">
            {(error as Error).message.includes("404") ||
            (error as Error).message.includes("not found")
              ? "Trace not found"
              : "Failed to load trace"}
          </h2>
          <p className="text-sm text-red-600 mt-2">
            {(error as Error).message}
          </p>
          <Link
            to="/"
            className="inline-block mt-4 text-sm text-blue-600 hover:text-blue-800"
          >
            Back to list
          </Link>
        </div>
      </div>
    );
  }

  if (!trace) return null;

  const tabs: { key: Tab; label: string }[] = [
    { key: "overview", label: "Overview" },
    { key: "timeline", label: "Timeline" },
    { key: "charts", label: "Charts" },
  ];

  return (
    <div className="max-w-7xl mx-auto space-y-4">
      <TraceHeader trace={trace} />

      <div className="border-b border-gray-200">
        <nav className="flex gap-6">
          {tabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`pb-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === tab.key
                  ? "border-blue-600 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {activeTab === "overview" && (
        <div className="flex gap-4" style={{ minHeight: "500px" }}>
          <div className="w-2/5 overflow-auto border border-gray-200 rounded-lg">
            <SpanTree
              rootSpan={trace.root_span}
              selectedSpan={selectedSpan}
              onSelectSpan={setSelectedSpan}
            />
          </div>
          <div className="w-3/5 overflow-auto border border-gray-200 rounded-lg">
            <SpanDetail span={selectedSpan} />
          </div>
        </div>
      )}

      {activeTab === "timeline" && <Timeline trace={trace} />}

      {activeTab === "charts" && (
        <div className="space-y-6">
          <TokenChart trace={trace} />
          <DurationChart trace={trace} />
        </div>
      )}
    </div>
  );
}
