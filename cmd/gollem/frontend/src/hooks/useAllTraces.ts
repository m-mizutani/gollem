import { useQuery } from "@tanstack/react-query";
import { listAllTraces } from "../api/client";

export function useAllTraces() {
  return useQuery({
    queryKey: ["allTraces"],
    queryFn: listAllTraces,
  });
}
