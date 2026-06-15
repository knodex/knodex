// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAuditEvent,
  useAuditStats,
  useAuditConfig,
} from "./useAudit";
import * as auditApi from "@/api/audit";
import type { ReactNode } from "react";

vi.mock("@/api/audit");

const originalEnterprise = globalThis.__ENTERPRISE__;

function createWrapper() {
  const queryClient = new QueryClient({
    // Let the hooks' own retry predicate run (don't override at the client level).
    defaultOptions: { queries: {} },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function make403(): Error {
  return Object.assign(new Error("Forbidden"), { response: { status: 403 } });
}

describe("Audit hooks — 403 retry predicate (no retry)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  it("useAuditEvent does not retry on 403", async () => {
    vi.mocked(auditApi.getAuditEvent).mockRejectedValue(make403());
    const { result } = renderHook(() => useAuditEvent("id-1"), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isError).toBe(true), {
      timeout: 5000,
    });
    expect(auditApi.getAuditEvent).toHaveBeenCalledTimes(1);
  });

  it("useAuditStats does not retry on 403", async () => {
    vi.mocked(auditApi.getAuditStats).mockRejectedValue(make403());
    const { result } = renderHook(() => useAuditStats(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isError).toBe(true), {
      timeout: 5000,
    });
    expect(auditApi.getAuditStats).toHaveBeenCalledTimes(1);
  });

  it("useAuditConfig does not retry on 403", async () => {
    vi.mocked(auditApi.getAuditConfig).mockRejectedValue(make403());
    const { result } = renderHook(() => useAuditConfig(), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.isError).toBe(true), {
      timeout: 5000,
    });
    expect(auditApi.getAuditConfig).toHaveBeenCalledTimes(1);
  });
});
