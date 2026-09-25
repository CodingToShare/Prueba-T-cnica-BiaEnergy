"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { api } from "@/lib/api/endpoints";
import type { DashboardSummary } from "@/lib/api/types";
import { queryKeys } from "@/lib/query/keys";

import { POLL_INTERVAL_MS, isActive } from "./state";

/**
 * Runs and follows an analysis:
 * - `start` calls POST /api/v1/ai/analyze (the backend returns the active run
 *   instead of creating a duplicate);
 * - the run is polled only while QUEUED or RUNNING, and polling stops on
 *   COMPLETED or FAILED — even when the very first poll is already final;
 * - a run already active when the page loads (dashboard `active_analysis`)
 *   is picked up, so a refresh does not lose the workflow;
 * - on completion, dashboard, meters and anomalies are refreshed once.
 */
export function useAnalysis(dashboard: DashboardSummary | undefined) {
  const queryClient = useQueryClient();
  const [trackedId, setTrackedId] = useState<number | null>(null);
  const runId = trackedId ?? dashboard?.active_analysis?.id ?? null;

  const run = useQuery({
    queryKey: queryKeys.analysis(runId ?? 0),
    queryFn: ({ signal }) => api.analysis(runId as number, signal),
    enabled: runId !== null,
    staleTime: 0,
    refetchInterval: (query) => {
      if (query.state.status === "error") return false;
      const status = query.state.data?.status;
      return status !== undefined && isActive(status) ? POLL_INTERVAL_MS : false;
    },
  });

  const start = useMutation({
    mutationFn: () => api.analyze(),
    onSuccess: (accepted) => {
      queryClient.setQueryData(queryKeys.analysis(accepted.analysis_id), accepted);
      setTrackedId(accepted.analysis_id);
    },
  });

  const refreshedFor = useRef<number | null>(null);
  const data = run.data;
  useEffect(() => {
    if (data?.status === "COMPLETED" && refreshedFor.current !== data.analysis_id) {
      refreshedFor.current = data.analysis_id;
      void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
      void queryClient.invalidateQueries({ queryKey: queryKeys.meters.all });
      void queryClient.invalidateQueries({ queryKey: queryKeys.anomalies.all });
    }
  }, [data, queryClient]);

  const active = data !== undefined ? isActive(data.status) : dashboard?.active_analysis != null;

  return {
    run: data,
    runError: run.error,
    retryStatus: () => void run.refetch(),
    retryingStatus: run.isFetching,
    start: () => start.mutate(),
    starting: start.isPending,
    startError: start.error,
    busy: start.isPending || active,
  };
}
