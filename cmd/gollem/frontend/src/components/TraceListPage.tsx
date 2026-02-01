import { useState, useCallback } from "react";
import { useTraces } from "../hooks/useTraces";
import TraceIDInput from "./TraceIDInput";
import TraceListTable from "./TraceListTable";

const PAGE_SIZE = 20;

export default function TraceListPage() {
  const [pageTokens, setPageTokens] = useState<string[]>([""]);
  const [currentPage, setCurrentPage] = useState(0);

  const currentToken = pageTokens[currentPage] || "";
  const { data, isLoading, error } = useTraces(PAGE_SIZE, currentToken);

  const handleNextPage = useCallback(() => {
    if (data?.next_page_token) {
      const nextPage = currentPage + 1;
      setPageTokens((prev) => {
        const updated = [...prev];
        updated[nextPage] = data.next_page_token!;
        return updated;
      });
      setCurrentPage(nextPage);
    }
  }, [data, currentPage]);

  const handlePrevPage = useCallback(() => {
    if (currentPage > 0) {
      setCurrentPage(currentPage - 1);
    }
  }, [currentPage]);

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
        traces={data?.traces || []}
        isLoading={isLoading}
        currentPage={currentPage}
        hasNextPage={!!data?.next_page_token}
        hasPrevPage={currentPage > 0}
        onNextPage={handleNextPage}
        onPrevPage={handlePrevPage}
      />
    </div>
  );
}
