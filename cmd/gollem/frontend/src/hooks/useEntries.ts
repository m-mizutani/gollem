import { useQuery } from "@tanstack/react-query";
import { listAllEntries } from "../api/client";

export function useEntries(path: string) {
  return useQuery({
    queryKey: ["entries", path],
    queryFn: () => listAllEntries(path),
  });
}
