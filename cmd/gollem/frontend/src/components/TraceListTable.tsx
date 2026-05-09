import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import type { Entry } from "../api/types";
import { formatBytes, formatRelativeTime, formatDateTime } from "../utils/format";

const PAGE_SIZE = 20;

interface TraceListTableProps {
  entries: Entry[];
  currentPath: string;
  isLoading: boolean;
}

type SortField = "name" | "updated_at" | "size";
type SortOrder = "asc" | "desc";

function joinPath(base: string, name: string): string {
  return base ? `${base}/${name}` : name;
}

function dirHref(base: string, name: string): string {
  return `/?path=${encodeURIComponent(joinPath(base, name))}`;
}

function fileHref(base: string, name: string): string {
  const full = joinPath(base, name);
  return `/traces/${full
    .split("/")
    .map((s) => encodeURIComponent(s))
    .join("/")}`;
}

export default function TraceListTable({
  entries,
  currentPath,
  isLoading,
}: TraceListTableProps) {
  const [filter, setFilter] = useState("");
  const [sortField, setSortField] = useState<SortField>("updated_at");
  const [sortOrder, setSortOrder] = useState<SortOrder>("desc");
  const [showRelativeTime, setShowRelativeTime] = useState(true);
  const [currentPage, setCurrentPage] = useState(0);

  const filteredAndSorted = useMemo(() => {
    let result = entries;
    if (filter) {
      const lower = filter.toLowerCase();
      result = result.filter((e) => e.name.toLowerCase().includes(lower));
    }
    return [...result].sort((a, b) => {
      // Directories always first.
      if (a.kind !== b.kind) {
        return a.kind === "dir" ? -1 : 1;
      }
      let cmp = 0;
      if (sortField === "name" || a.kind === "dir") {
        cmp = a.name.localeCompare(b.name);
      } else if (sortField === "updated_at") {
        const at = a.updated_at ? new Date(a.updated_at).getTime() : 0;
        const bt = b.updated_at ? new Date(b.updated_at).getTime() : 0;
        cmp = at - bt;
      } else {
        cmp = (a.size ?? 0) - (b.size ?? 0);
      }
      return sortOrder === "asc" ? cmp : -cmp;
    });
  }, [entries, filter, sortField, sortOrder]);

  const totalPages = Math.max(1, Math.ceil(filteredAndSorted.length / PAGE_SIZE));
  const safePage = Math.min(currentPage, totalPages - 1);
  const pageEntries = filteredAndSorted.slice(
    safePage * PAGE_SIZE,
    (safePage + 1) * PAGE_SIZE
  );

  const toggleSort = (field: SortField) => {
    setCurrentPage(0);
    if (sortField === field) {
      setSortOrder(sortOrder === "asc" ? "desc" : "asc");
    } else {
      setSortField(field);
      setSortOrder("desc");
    }
  };

  const sortIndicator = (field: SortField) => {
    if (sortField !== field) return "";
    return sortOrder === "asc" ? " ↑" : " ↓";
  };

  if (isLoading) {
    return (
      <div className="bg-white rounded-lg border border-gray-200">
        <div className="p-8 text-center text-gray-500">Loading entries...</div>
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg border border-gray-200">
      <div className="p-3 border-b border-gray-200">
        <input
          type="text"
          placeholder="Filter by name..."
          value={filter}
          onChange={(e) => {
            setFilter(e.target.value);
            setCurrentPage(0);
          }}
          className="w-full px-3 py-1.5 text-sm border border-gray-300 rounded focus:outline-none focus:ring-1 focus:ring-blue-500"
        />
      </div>

      <table className="w-full text-sm">
        <thead className="bg-gray-50 text-gray-600">
          <tr>
            <th
              className="text-left px-4 py-2 font-medium cursor-pointer select-none hover:text-gray-900"
              onClick={() => toggleSort("name")}
            >
              Name{sortIndicator("name")}
            </th>
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
          {pageEntries.length === 0 ? (
            <tr>
              <td colSpan={3} className="px-4 py-8 text-center text-gray-500">
                No entries found
              </td>
            </tr>
          ) : (
            pageEntries.map((entry) => {
              const isDir = entry.kind === "dir";
              const href = isDir
                ? dirHref(currentPath, entry.name)
                : fileHref(currentPath, entry.name);
              const displayName =
                !isDir && entry.name.length > 36
                  ? entry.name.slice(0, 8) + "..."
                  : entry.name;
              return (
                <tr
                  key={`${entry.kind}:${entry.name}`}
                  className="border-t border-gray-100 hover:bg-gray-50"
                >
                  <td className="px-4 py-2">
                    <Link
                      to={href}
                      className="text-blue-600 hover:text-blue-800 font-mono inline-flex items-center gap-1.5"
                      title={entry.name}
                    >
                      <span aria-hidden className="text-gray-400">
                        {isDir ? "\u{1F4C1}" : "\u{1F4C4}"}
                      </span>
                      <span>
                        {displayName}
                        {isDir ? "/" : ""}
                      </span>
                    </Link>
                  </td>
                  <td
                    className="px-4 py-2 text-gray-600 cursor-pointer"
                    onClick={() => setShowRelativeTime(!showRelativeTime)}
                    title={entry.updated_at ? formatDateTime(entry.updated_at) : ""}
                  >
                    {entry.updated_at
                      ? showRelativeTime
                        ? formatRelativeTime(entry.updated_at)
                        : formatDateTime(entry.updated_at)
                      : "-"}
                  </td>
                  <td className="px-4 py-2 text-gray-600">
                    {isDir ? "-" : formatBytes(entry.size ?? 0)}
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>

      <div className="px-4 py-3 border-t border-gray-200 flex items-center justify-between">
        <span className="text-sm text-gray-500">
          {filteredAndSorted.length} entries — Page {safePage + 1} of {totalPages}
        </span>
        <div className="flex gap-2">
          <button
            onClick={() => setCurrentPage(safePage - 1)}
            disabled={safePage <= 0}
            className="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Previous
          </button>
          <button
            onClick={() => setCurrentPage(safePage + 1)}
            disabled={safePage >= totalPages - 1}
            className="px-3 py-1 text-sm border border-gray-300 rounded hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
}
