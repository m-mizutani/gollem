import { useAllTraces } from "../hooks/useAllTraces";
import TraceIDInput from "./TraceIDInput";
import TraceListTable from "./TraceListTable";

export default function TraceListPage() {
  const { data: traces, isLoading, error } = useAllTraces();

  return (
    <div className="max-w-6xl mx-auto space-y-6">
      <TraceIDInput />

      {error && (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4 text-red-700">
          <p className="font-medium">Failed to load traces</p>
          <p className="text-sm mt-1">{(error as Error).message}</p>
        </div>
      )}

      <TraceListTable
        traces={traces || []}
        isLoading={isLoading}
      />
    </div>
  );
}
