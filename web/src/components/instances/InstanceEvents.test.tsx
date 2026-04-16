// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { InstanceEvents } from "./InstanceEvents";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import * as useHistoryHooks from "@/hooks/useHistory";
import type { KubernetesEvent } from "@/types/history";

// Mock the hooks module
vi.mock("@/hooks/useHistory");

// Helper to render with React Query provider
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>{children}</TooltipProvider>
    </QueryClientProvider>
  );
}

// Mock data using the actual KubernetesEvent type
const mockNormalEvent: KubernetesEvent = {
  lastSeen: new Date().toISOString(),
  firstSeen: new Date(Date.now() - 60000).toISOString(),
  type: "Normal",
  reason: "Started",
  object: "WebApp/test-instance",
  message: "Container started successfully",
  count: 1,
  source: "kubelet",
};

const mockWarningEvent: KubernetesEvent = {
  lastSeen: new Date(Date.now() - 60000).toISOString(),
  firstSeen: new Date(Date.now() - 120000).toISOString(),
  type: "Warning",
  reason: "Unhealthy",
  object: "WebApp/test-instance",
  message: "Health check failed",
  count: 3,
  source: "health-controller",
};

function mockHook(overrides: Partial<ReturnType<typeof useHistoryHooks.useInstanceEvents>>) {
  vi.mocked(useHistoryHooks.useInstanceEvents).mockReturnValue({
    data: undefined,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...overrides,
  } as ReturnType<typeof useHistoryHooks.useInstanceEvents>);
}

describe("InstanceEvents", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("renders loading spinner when loading", () => {
    mockHook({ isLoading: true });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    const spinner = document.querySelector(".animate-spin");
    expect(spinner).toBeTruthy();
  });

  it("renders error state with retry button", () => {
    const mockError = new Error("Network error");
    mockHook({ error: mockError });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText(/Failed to load events/i)).toBeTruthy();
    expect(screen.getByText(/Network error/i)).toBeTruthy();
    expect(screen.getByRole("button", { name: /Retry/i })).toBeTruthy();
  });

  it("renders empty state when no events", () => {
    mockHook({ data: { events: [] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText(/No events/i)).toBeTruthy();
  });

  it("renders events list with correct content", () => {
    mockHook({ data: { events: [mockNormalEvent, mockWarningEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText(/Container started successfully/i)).toBeTruthy();
    expect(screen.getByText(/Health check failed/i)).toBeTruthy();
  });

  it("shows correct severity styling for Warning vs Normal events", () => {
    mockHook({ data: { events: [mockNormalEvent, mockWarningEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // Warning text should be rendered with status-warning class
    const warningText = screen.getByText("Warning");
    expect(warningText).toHaveClass("text-status-warning");

    // Normal text should be rendered with muted class
    const normalText = screen.getByText("Normal");
    expect(normalText).toHaveClass("text-muted-foreground");
  });

  it("filter buttons exist and are clickable", () => {
    mockHook({ data: { events: [mockNormalEvent, mockWarningEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // Check for filter buttons with counts
    expect(screen.getByText("All (2)")).toBeTruthy();
    expect(screen.getByText("Warning (1)")).toBeTruthy();
    expect(screen.getByText("Normal (1)")).toBeTruthy();
  });

  it("filters events when filter button is clicked", () => {
    mockHook({ data: { events: [mockNormalEvent, mockWarningEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // Click Warning filter
    fireEvent.click(screen.getByText("Warning (1)"));

    // Should only show warning event
    expect(screen.getByText("Health check failed")).toBeTruthy();
    expect(screen.queryByText("Container started successfully")).not.toBeTruthy();
  });

  it("shows event reason and object in table", () => {
    mockHook({ data: { events: [mockNormalEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText("Started")).toBeTruthy();
    expect(screen.getByText("WebApp/test-instance")).toBeTruthy();
  });

  it("shows event count in header badge", () => {
    mockHook({ data: { events: [mockNormalEvent, mockWarningEvent] } });

    render(
      <InstanceEvents namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // The filtered count badge
    expect(screen.getByText("2")).toBeTruthy();
  });
});
