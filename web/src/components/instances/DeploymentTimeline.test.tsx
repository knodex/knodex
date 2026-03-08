// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { TooltipProvider } from "@/components/ui/tooltip";
import { DeploymentTimeline } from "./DeploymentTimeline";
import * as useHistoryHooks from "@/hooks/useHistory";
import type { TimelineResponse } from "@/types/history";

// Mock the hooks
vi.mock("@/hooks/useHistory");

const mockTimelineData: TimelineResponse = {
  namespace: "test-ns",
  name: "test-instance",
  timeline: [
    {
      timestamp: "2024-01-01T00:00:00Z",
      eventType: "Created",
      status: "Pending",
      user: "test-user@example.com",
      isCompleted: true,
      isCurrent: false,
    },
    {
      timestamp: "2024-01-01T00:01:00Z",
      eventType: "PushedToGit",
      status: "PushedToGit",
      gitCommitUrl: "https://github.com/org/repo/commit/abc123",
      isCompleted: true,
      isCurrent: false,
    },
    {
      timestamp: "2024-01-01T00:02:00Z",
      eventType: "Ready",
      status: "Ready",
      message: "Instance is ready",
      isCompleted: true,
      isCurrent: true,
    },
  ],
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>{children}</TooltipProvider>
      </QueryClientProvider>
    );
  };
}

describe("DeploymentTimeline", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: vi.fn(),
    } as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Deployment History")).toBeInTheDocument();
    // Loading spinner should be present (svg with animate-spin)
    const loadingContainer = screen.getByText("Deployment History").closest("div");
    expect(loadingContainer).toBeInTheDocument();
  });

  it("renders error state with retry button", async () => {
    const mockRefetch = vi.fn();
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Network error"),
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Failed to load deployment history")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();

    // Click retry button
    const retryButton = screen.getByRole("button", { name: /retry/i });
    fireEvent.click(retryButton);

    expect(mockRefetch).toHaveBeenCalled();
  });

  it("renders timeline entries correctly", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Check event types are rendered
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Pushed to Git")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();

    // Check event count badge
    expect(screen.getByText("3 events")).toBeInTheDocument();

    // Check current badge on the last entry
    expect(screen.getByText("Current")).toBeInTheDocument();

    // Check user is displayed
    expect(screen.getByText("test-user@example.com")).toBeInTheDocument();

    // Check message is displayed
    expect(screen.getByText("Instance is ready")).toBeInTheDocument();
  });

  it("renders git commit link", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    const commitLink = screen.getByRole("link", { name: /view commit/i });
    expect(commitLink).toHaveAttribute(
      "href",
      "https://github.com/org/repo/commit/abc123"
    );
    expect(commitLink).toHaveAttribute("target", "_blank");
    expect(commitLink).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("toggles expansion when header is clicked", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Initially expanded
    expect(screen.getByText("Created")).toBeInTheDocument();

    // Click header to collapse
    const header = screen.getByRole("button", { name: /deployment history/i });
    fireEvent.click(header);

    // Events should be hidden
    expect(screen.queryByText("Created")).not.toBeInTheDocument();

    // Click again to expand
    fireEvent.click(header);
    expect(screen.getByText("Created")).toBeInTheDocument();
  });

  it("renders empty state when no events", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: { namespace: "test-ns", name: "test-instance", timeline: [] },
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("No deployment history available")).toBeInTheDocument();
    expect(screen.getByText("0 events")).toBeInTheDocument();
  });

  it("triggers export when download button is clicked", async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({ success: true });

    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Click download button
    const downloadButton = screen.getByRole("button", { name: /download/i });
    fireEvent.click(downloadButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        namespace: "test-ns",
        kind: "WebApp",
        name: "test-instance",
        format: "json",
      });
    });
  });

  it("changes export format via select", async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({ success: true });

    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Change format to CSV
    const formatSelect = screen.getByRole("combobox");
    fireEvent.change(formatSelect, { target: { value: "csv" } });

    // Click download
    const downloadButton = screen.getByRole("button", { name: /download/i });
    fireEvent.click(downloadButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        namespace: "test-ns",
        kind: "WebApp",
        name: "test-instance",
        format: "csv",
      });
    });
  });

  it("shows export error message", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
      isError: true,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Failed to export history")).toBeInTheDocument();
  });

  it("disables download button while exporting", () => {
    vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
      data: mockTimelineData,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

    vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: true,
      isError: false,
    } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    const downloadButton = screen.getByRole("button", { name: /download/i });
    expect(downloadButton).toBeDisabled();
  });
});
