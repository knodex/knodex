import { useQuery, useMutation, useQueryClient, keepPreviousData } from "@tanstack/react-query";
import {
  getComplianceSummary,
  listConstraintTemplates,
  getConstraintTemplate,
  listConstraints,
  getConstraint,
  listViolations,
  updateConstraintEnforcement,
  createConstraint,
  getViolationHistoryCount,
  exportViolationHistory,
  downloadBlob,
  isEnterpriseRequired,
  isGatekeeperUnavailable,
} from "@/api/compliance";
import type {
  ConstraintListParams,
  ViolationListParams,
  TemplateListParams,
  EnforcementAction,
  CreateConstraintRequest,
  ViolationHistoryCountParams,
  ViolationHistoryExportParams,
} from "@/types/compliance";

/**
 * Check if enterprise features are enabled
 * Uses the __ENTERPRISE__ global variable defined in vite.config.ts
 *
 * For E2E testing, use ENTERPRISE_BUILD=true when deploying:
 *   ENTERPRISE_BUILD=true make qa-deploy
 */
export function isEnterprise(): boolean {
  return typeof __ENTERPRISE__ !== "undefined" && __ENTERPRISE__;
}

/**
 * Hook for fetching compliance summary statistics
 * Only enabled when enterprise features are active
 */
export function useComplianceSummary() {
  return useQuery({
    queryKey: ["compliance", "summary"],
    queryFn: getComplianceSummary,
    enabled: isEnterprise(),
    staleTime: 30 * 1000, // 30 seconds
    retry: (failureCount, error) => {
      // Don't retry on 402 Payment Required or 503 Service Unavailable
      if (isEnterpriseRequired(error) || isGatekeeperUnavailable(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching the violation count (for sidebar badge)
 * Uses the summary endpoint but only returns the count
 */
export function useViolationCount() {
  const { data, ...rest } = useComplianceSummary();
  return {
    data: data?.totalViolations,
    ...rest,
  };
}

/**
 * Hook for fetching paginated constraint templates list
 * Only enabled when enterprise features are active
 */
export function useConstraintTemplates(params?: TemplateListParams) {
  return useQuery({
    queryKey: ["compliance", "templates", params],
    queryFn: () => listConstraintTemplates(params),
    enabled: isEnterprise(),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000,
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching a single constraint template by name
 */
export function useConstraintTemplate(name: string) {
  return useQuery({
    queryKey: ["compliance", "template", name],
    queryFn: () => getConstraintTemplate(name),
    enabled: isEnterprise() && !!name,
    staleTime: 60 * 1000, // 1 minute - templates change infrequently
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching paginated constraints list with optional filtering
 * Only enabled when enterprise features are active
 */
export function useConstraints(params?: ConstraintListParams) {
  return useQuery({
    queryKey: ["compliance", "constraints", params],
    queryFn: () => listConstraints(params),
    enabled: isEnterprise(),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000,
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching a single constraint by kind and name
 */
export function useConstraint(kind: string, name: string) {
  return useQuery({
    queryKey: ["compliance", "constraint", kind, name],
    queryFn: () => getConstraint(kind, name),
    enabled: isEnterprise() && !!kind && !!name,
    staleTime: 30 * 1000,
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching paginated violations list with optional filtering
 * Only enabled when enterprise features are active
 */
export function useViolations(params?: ViolationListParams) {
  return useQuery({
    queryKey: ["compliance", "violations", params],
    queryFn: () => listViolations(params),
    enabled: isEnterprise(),
    placeholderData: keepPreviousData,
    staleTime: 30 * 1000,
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

/**
 * Hook for fetching recent violations (limited to top N)
 * Used on the dashboard overview
 */
export function useRecentViolations(limit: number = 10) {
  return useViolations({ pageSize: limit, page: 1 });
}

/**
 * Parameters for updating constraint enforcement
 */
export interface UpdateEnforcementParams {
  kind: string;
  name: string;
  enforcementAction: EnforcementAction;
}

/**
 * Hook for updating a constraint's enforcement action
 * Invalidates relevant queries on success
 */
export function useUpdateConstraintEnforcement() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ kind, name, enforcementAction }: UpdateEnforcementParams) =>
      updateConstraintEnforcement(kind, name, enforcementAction),
    onSuccess: (data, variables) => {
      // Invalidate the specific constraint query
      queryClient.invalidateQueries({
        queryKey: ["compliance", "constraint", variables.kind, variables.name],
      });
      // Invalidate the constraints list query (enforcement counts may change)
      queryClient.invalidateQueries({
        queryKey: ["compliance", "constraints"],
      });
      // Invalidate the summary query (byEnforcement counts may change)
      queryClient.invalidateQueries({
        queryKey: ["compliance", "summary"],
      });
    },
  });
}

/**
 * Hook for creating a new constraint from a ConstraintTemplate
 * Invalidates relevant queries on success
 */
export function useCreateConstraint() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: CreateConstraintRequest) => createConstraint(request),
    onSuccess: (data) => {
      // Invalidate the constraints list query (new constraint added)
      queryClient.invalidateQueries({
        queryKey: ["compliance", "constraints"],
      });
      // Invalidate the summary query (constraint count changed)
      queryClient.invalidateQueries({
        queryKey: ["compliance", "summary"],
      });
      // Invalidate the specific constraint query if it was previously queried
      queryClient.invalidateQueries({
        queryKey: ["compliance", "constraint", data.kind, data.name],
      });
    },
  });
}

/**
 * Hook for fetching violation history count (preview count for export dialog)
 */
export function useViolationHistoryCount(params?: ViolationHistoryCountParams) {
  return useQuery({
    queryKey: ["compliance", "violations", "history", "count", params],
    queryFn: () => getViolationHistoryCount(params),
    enabled: isEnterprise() && !!params?.since,
    staleTime: 30 * 1000,
    retry: (failureCount, error) => {
      if (isEnterpriseRequired(error)) return false;
      return failureCount < 2;
    },
  });
}

/**
 * Hook for exporting violation history as CSV
 */
export function useExportViolationHistory() {
  return useMutation({
    mutationFn: async (params: ViolationHistoryExportParams) => {
      const { blob, filename } = await exportViolationHistory(params);
      downloadBlob(blob, filename);
    },
  });
}
