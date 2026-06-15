// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import AuditPage from "./AuditPage";
import { ApiError } from "@/api/client";
import * as useAuditModule from "@/hooks/useAudit";
import * as useComplianceModule from "@/hooks/useCompliance";
import type { AuditEvent, AuditEventList, AuditStats } from "@/types/audit";

vi.mock("@/hooks/useAudit", () => ({
  useAuditEvents: vi.fn(),
  useAuditStats: vi.fn(),
}));

vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: vi.fn(() => true),
}));

vi.mock("@/api/audit", () => ({
  fetchAllAuditEvents: vi.fn(),
}));

// AuditFilters renders a project <Select> backed by useProjects — stub it so the
// real filter row mounts without firing a network query.
vi.mock("@/hooks/useProjects", () => ({
  useProjects: vi.fn(() => ({ data: { items: [], totalCount: 0 }, isLoading: false })),
}));

const createTestQueryClient = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderPage(initialRoute = "/audit") {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter initialEntries={[initialRoute]}>
          <AuditPage />
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

/** Minimal but complete AuditEvent so AuditEventsTable/ResultBadge render. */
function makeEvent(id: string, result: string): AuditEvent {
  return {
    id,
    timestamp: "2026-05-28T10:00:00Z",
    userId: `user-${id}`,
    userEmail: `user-${id}@test.local`,
    sourceIP: "10.0.0.1",
    action: "get",
    resource: "instances",
    name: `obj-${id}`,
    project: "alpha",
    namespace: "alpha-ns",
    requestId: `req-${id}`,
    result,
  };
}

const mockStats: AuditStats = {
  totalEvents: 120,
  eventsToday: 4,
  topUsers: [],
  deniedAttempts: 2,
  byActionToday: {},
  byResultToday: {},
};

function mockStatsHook() {
  vi.mocked(useAuditModule.useAuditStats).mockReturnValue({
    data: mockStats,
    isLoading: false,
    error: null,
  } as unknown as ReturnType<typeof useAuditModule.useAuditStats>);
}

function mockEventsHook(data: Partial<AuditEventList> | undefined, isLoading = false) {
  vi.mocked(useAuditModule.useAuditEvents).mockReturnValue({
    data,
    isLoading,
    error: null,
  } as unknown as ReturnType<typeof useAuditModule.useAuditEvents>);
}

function mockEventsHookError(error: unknown) {
  vi.mocked(useAuditModule.useAuditEvents).mockReturnValue({
    data: undefined,
    isLoading: false,
    error,
  } as unknown as ReturnType<typeof useAuditModule.useAuditEvents>);
}

describe("AuditPage — 403 Access Denied", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
    mockStatsHook();
  });

  it("renders Access Denied when the events query rejects with an ApiError 403 (interceptor shape)", () => {
    // The apiClient response interceptor rethrows backend errors as ApiError
    // with a flat `.status` — the shape useAuditEvents actually surfaces.
    mockEventsHookError(new ApiError("FORBIDDEN", "permission denied", 403));

    renderPage();

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    expect(
      screen.getByText("You do not have permission to view audit events.")
    ).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders Access Denied for a raw AxiosError-shaped 403 (interceptor-bypass fallback)", () => {
    mockEventsHookError({ response: { status: 403 } });

    renderPage();

    expect(screen.getByText("Access Denied")).toBeInTheDocument();
  });

  it("does NOT render Access Denied for a non-403 ApiError", () => {
    mockEventsHookError(new ApiError("INTERNAL_ERROR", "boom", 500));

    renderPage();

    expect(screen.queryByText("Access Denied")).not.toBeInTheDocument();
  });
});

describe("AuditPage — ListFooter", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(true);
    mockStatsHook();
  });

  it("renders the footer with counts over the visible page (page length, NOT server total)", async () => {
    // 4 events on the current page; server reports 120 total across all pages.
    mockEventsHook({
      events: [
        makeEvent("1", "success"),
        makeEvent("2", "success"),
        makeEvent("3", "denied"),
        makeEvent("4", "error"),
      ],
      total: 120,
      page: 1,
      pageSize: 50,
    });

    renderPage();

    const footer = await screen.findByTestId("audit-list-footer");
    // Reflects the page length (4), not the server total (120) — guards the
    // "events shown ≠ total" decision.
    expect(footer).toHaveTextContent("4 events shown");
    expect(footer).toHaveTextContent("2 success");
    expect(footer).toHaveTextContent("2 denied / errored");
    expect(footer).not.toHaveTextContent("120 events shown");
  });

  it("does NOT render the footer when the current page is empty", async () => {
    mockEventsHook({ events: [], total: 0, page: 1, pageSize: 50 });

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("No audit events found")).toBeInTheDocument();
    });
    expect(screen.queryByTestId("audit-list-footer")).not.toBeInTheDocument();
  });

  it("does NOT render the footer while loading", () => {
    mockEventsHook({ events: [], total: 0, page: 1, pageSize: 50 }, true);

    renderPage();

    expect(screen.queryByTestId("audit-list-footer")).not.toBeInTheDocument();
  });

  it("does NOT render the footer in the non-enterprise (EnterpriseRequired) gate", () => {
    vi.mocked(useComplianceModule.isEnterprise).mockReturnValue(false);
    mockEventsHook(undefined);

    renderPage();

    expect(screen.getByText("Enterprise Feature")).toBeInTheDocument();
    expect(screen.getByText(/Audit Trail requires an Enterprise license/)).toBeInTheDocument();
    expect(screen.queryByTestId("audit-list-footer")).not.toBeInTheDocument();
  });
});
