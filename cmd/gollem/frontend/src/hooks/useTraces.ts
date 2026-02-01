import { useQuery } from "@tanstack/react-query";
import { listTraces } from "../api/client";

export function useTraces(pageSize: number = 20, pageToken: string = "") {
  return useQuery({
    queryKey: ["traces", pageSize, pageToken],
    queryFn: () => listTraces(pageSize, pageToken),
  });
}
