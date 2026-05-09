import { useSearchParams } from "react-router-dom";
import { useEntries } from "../hooks/useEntries";
import Breadcrumb from "./Breadcrumb";
import TraceIDInput from "./TraceIDInput";
import TraceListTable from "./TraceListTable";

export default function TraceListPage() {
  const [searchParams] = useSearchParams();
  const path = searchParams.get("path") || "";

  const { data: entries, isLoading, error } = useEntries(path);

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      <TraceIDInput currentPath={path} />

      <Breadcrumb path={path} />

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Failed to load entries</p>
          <p className="text-sm mt-1">{(error as Error).message}</p>
        </div>
      )}

      <TraceListTable
        entries={entries || []}
        currentPath={path}
        isLoading={isLoading}
      />
    </div>
  );
}
