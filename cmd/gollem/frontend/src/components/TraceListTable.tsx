import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import type { TraceSummary } from "../api/types";
import { formatBytes, formatRelativeTime, formatDateTime } from "../utils/format";

interface TraceListTableProps {
  traces: TraceSummary[];
  isLoading: boolean;
  currentPage: number;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  onNextPage: () => void;
  onPrevPage: () => void;
}

type SortField = "updated_at" | "size";
type SortOrder = "asc" | "desc";

export default function TraceListTable({
  traces,
  isLoading,
  currentPage,
  hasNextPage,
  hasPrevPage,
  onNextPage,
  onPrevPage,
}: TraceListTableProps) {
  const [filter, setFilter] = useState("");
  const [sortField, setSortField] = useState<SortField>("updated_at");
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");
  const [showRelativeTime, setShowRelativeTime] = useState(true);

  const filteredAndSorted = useMemo(() => {
    let result = traces;
    if (filter) {
      const lower = filter.toLowerCase();
      result = result.filter((t) =>
        t.trace_id.toLowerCase().includes(lower)
      );
    }
    return [...result].sort((a, b) => {
      let cmp = 0;
      if (sortField === "updated_at") {
        cmp =
          new Date(a.updated_at).getTime() -
          new Date(b.updated_at).getTime();
      } else {
        cmp = a.size - b.size;
      }
      return sortOrder === "asc" ? cmp : -cmp;
    });
  }, [traces, filter, sortField, sortOrder]);

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortOrder(sortOrder === "asc" ? "desc" : "asc");
    } else {
      setSortField(field);
      setSortOrder("desc");
    }
  };

  const sortIndicator = (field: SortField) => {
    if (sortField !== field) return "";
    return sortOrder === "asc" ? " \u2191" : " \u2193";
  };

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg border border-gray-200">
        <div className="p-8 text-center text-gray-500">Loading traces...</div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg border border-gray-200">
      <div className="p-3 border-b border-gray-200">
        <input
          type="text"
          placeholder="Filter by Trace ID..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          className="w-full px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-gray-600">
          <tr>
            <th className="text-left px-4 py-2 font-medium">Trace ID</th>
            <th
              className="text-left px-4 py-2 font-medium cursor-pointer select-none hover:text-gray-900"
              onClick={() => toggleSort("updated_at")}
            >
              Updated{sortIndicator("updated_at")}
            </th>
            <th
              className="text-left px-4 py-2 font-medium cursor-pointer select-none hover:text-gray-900"
              onClick={() => toggleSort("size")}
            >
              Size{sortIndicator("size")}
            </th>
          </tr>
        </thead>
        <tbody>
          {filteredAndSorted.length === 0 ? (
            <tr>
              <td colSpan={3} className="px-4 py-8 text-center text-gray-500">
                No traces found
              </td>
            </tr>
          ) : (
            filteredAndSorted.map((trace) => (
              <tr
                key={trace.trace_id}
                className="border-t border-gray-100 hover:bg-gray-50"
              >
                <td className="px-4 py-2">
                  <Link
                    to={`/traces/${trace.trace_id}`}
                    className="text-blue-600 hover:text-blue-800 font-mono"
                    title={trace.trace_id}
                  >
                    {trace.trace_id.length > 36
                      ? trace.trace_id.slice(0, 8) + "..."
                      : trace.trace_id}
                  </Link>
                </td>
                <td
                  className="px-4 py-2 text-gray-600 cursor-pointer"
                  onClick={() => setShowRelativeTime(!showRelativeTime)}
                  title={formatDateTime(trace.updated_at)}
                >
                  {showRelativeTime
                    ? formatRelativeTime(trace.updated_at)
                    : formatDateTime(trace.updated_at)}
                </td>
                <td className="px-4 py-2 text-gray-600">
                  {formatBytes(trace.size)}
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>

      <div className="px-4 py-3 border-t border-gray-200 flex items-center justify-between">
        <span className="text-sm text-gray-500">Page {currentPage + 1}</span>
        <div className="flex gap-2">
          <button
            onClick={onPrevPage}
            disabled={!hasPrevPage}
            className="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Previous
          </button>
          <button
            onClick={onNextPage}
            disabled={!hasNextPage}
            className="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
