import { useQuery } from "@tanstack/react-query";
import { getTrace } from "../api/client";

export function useTrace(traceID: string) {
  return useQuery({
    queryKey: ["trace", traceID],
    queryFn: () => getTrace(traceID),
    enabled: !!traceID,
  });
}
