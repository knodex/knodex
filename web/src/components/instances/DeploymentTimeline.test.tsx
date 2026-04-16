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
  kind: "WebApp",
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

function setupMocks(
  timelineOverrides?: Partial<ReturnType<typeof useHistoryHooks.useInstanceTimeline>>,
  exportOverrides?: Partial<ReturnType<typeof useHistoryHooks.useExportHistory>>,
) {
  vi.mocked(useHistoryHooks.useInstanceTimeline).mockReturnValue({
    data: mockTimelineData,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    ...timelineOverrides,
  } as unknown as ReturnType<typeof useHistoryHooks.useInstanceTimeline>);

  vi.mocked(useHistoryHooks.useExportHistory).mockReturnValue({
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    ...exportOverrides,
  } as unknown as ReturnType<typeof useHistoryHooks.useExportHistory>);
}

describe("DeploymentTimeline", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state", () => {
    setupMocks({ data: undefined, isLoading: true });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Deployment History")).toBeInTheDocument();
    const loadingContainer = screen.getByText("Deployment History").closest("div");
    expect(loadingContainer).toBeInTheDocument();
  });

  it("renders error state with retry button", async () => {
    const mockRefetch = vi.fn();
    setupMocks({ data: undefined, isLoading: false, error: new Error("Network error"), refetch: mockRefetch });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Failed to load deployment history")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();

    const retryButton = screen.getByRole("button", { name: /retry/i });
    fireEvent.click(retryButton);

    expect(mockRefetch).toHaveBeenCalled();
  });

  it("renders timeline nodes with labels on the horizontal rail", () => {
    setupMocks();

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // All event labels should be visible as node labels
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Pushed to Git")).toBeInTheDocument();
    // "Ready" appears in both the node label and auto-selected detail card
    expect(screen.getAllByText("Ready").length).toBeGreaterThanOrEqual(1);

    // Event count badge
    expect(screen.getByText("3 events")).toBeInTheDocument();

    // "Now" marker at the end of the timeline
    expect(screen.getByText("Now")).toBeInTheDocument();
  });

  it("auto-selects the current event and shows its detail card", () => {
    setupMocks();

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // The current event (Ready) detail card should show automatically
    expect(screen.getByText("Instance is ready")).toBeInTheDocument();
  });

  it("shows detail for a clicked node", () => {
    setupMocks();

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Click the "Created" node to see its detail
    const createdNode = screen.getByRole("button", { name: /created$/i });
    fireEvent.click(createdNode);

    // Created event detail should now show user info
    expect(screen.getByText("test-user@example.com")).toBeInTheDocument();
  });

  it("renders git commit link when PushedToGit node is selected", () => {
    setupMocks();

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Click the PushedToGit node
    const pushNode = screen.getByRole("button", { name: /pushed to git/i });
    fireEvent.click(pushNode);

    const commitLink = screen.getByRole("link", { name: /view commit/i });
    expect(commitLink).toHaveAttribute(
      "href",
      "https://github.com/org/repo/commit/abc123"
    );
    expect(commitLink).toHaveAttribute("target", "_blank");
    expect(commitLink).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("toggles expansion when header is clicked", () => {
    setupMocks();

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // Initially expanded — node labels visible
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
    setupMocks({
      data: { namespace: "test-ns", kind: "WebApp", name: "test-instance", timeline: [] },
    });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("No deployment history available")).toBeInTheDocument();
    expect(screen.getByText("0 events")).toBeInTheDocument();
  });

  it("triggers export when download button is clicked", async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({ success: true });
    setupMocks({}, { mutateAsync: mockMutateAsync });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

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
    setupMocks({}, { mutateAsync: mockMutateAsync });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    const formatSelect = screen.getByRole("combobox");
    fireEvent.change(formatSelect, { target: { value: "csv" } });

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
    setupMocks({}, { isError: true });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Failed to export history")).toBeInTheDocument();
  });

  // =========================================================================
  // STORY-402: RevisionChanged rendering tests
  // =========================================================================

  it("applies primary color styling to RevisionChanged events", () => {
    const timelineWithRevision: TimelineResponse = {
      namespace: "test-ns",
      kind: "WebApp",
      name: "test-instance",
      timeline: [
        {
          timestamp: "2024-01-01T00:05:00Z",
          eventType: "RevisionChanged",
          status: "",
          user: "system",
          message: "RGD Revision 1 (initial)",
          isCompleted: true,
          isCurrent: true,
          revisionNumber: 1,
          previousRevision: 0,
        },
      ],
    };

    setupMocks({ data: timelineWithRevision });

    const { container } = render(
      <DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // The icon container for RevisionChanged must carry primary color classes
    const iconContainers = container.querySelectorAll(
      ".text-primary.bg-primary\\/10.border-primary\\/20"
    );
    expect(iconContainers.length).toBeGreaterThan(0);
  });

  it("renders RevisionChanged events with correct label", () => {
    const timelineWithRevision: TimelineResponse = {
      namespace: "test-ns",
      kind: "WebApp",
      name: "test-instance",
      timeline: [
        {
          timestamp: "2024-01-01T00:00:00Z",
          eventType: "Created",
          status: "Pending",
          user: "admin",
          isCompleted: true,
          isCurrent: false,
        },
        {
          timestamp: "2024-01-01T00:05:00Z",
          eventType: "RevisionChanged",
          status: "",
          user: "system",
          message: "RGD Revision 1 (initial)",
          isCompleted: true,
          isCurrent: false,
          revisionNumber: 1,
        },
        {
          timestamp: "2024-01-01T00:10:00Z",
          eventType: "RevisionChanged",
          status: "",
          user: "system",
          message: "RGD Revision 1 → 2",
          isCompleted: true,
          isCurrent: false,
          revisionNumber: 2,
          previousRevision: 1,
        },
        {
          timestamp: "2024-01-01T00:15:00Z",
          eventType: "Ready",
          status: "Ready",
          isCompleted: true,
          isCurrent: true,
        },
      ],
    };

    setupMocks({ data: timelineWithRevision });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // RevisionChanged events should render with "Revision Changed" label on the node
    const revisionLabels = screen.getAllByText("Revision Changed");
    expect(revisionLabels).toHaveLength(2);

    // Event count should include revision markers
    expect(screen.getByText("4 events")).toBeInTheDocument();

    // Click on first RevisionChanged node to see its detail
    const revisionNodes = screen.getAllByRole("button", { name: /revision changed/i });
    fireEvent.click(revisionNodes[0]);
    expect(screen.getByText("RGD Revision 1 (initial)")).toBeInTheDocument();

    // Click on second RevisionChanged node to see its detail
    fireEvent.click(revisionNodes[1]);
    expect(screen.getByText("RGD Revision 1 → 2")).toBeInTheDocument();
  });

  it("renders mixed deployment and revision events in correct order", () => {
    const mixedTimeline: TimelineResponse = {
      namespace: "test-ns",
      name: "test-instance",
      timeline: [
        {
          timestamp: "2024-01-01T00:00:00Z",
          eventType: "Created",
          status: "Pending",
          isCompleted: true,
          isCurrent: false,
        },
        {
          timestamp: "2024-01-01T00:03:00Z",
          eventType: "RevisionChanged",
          status: "",
          user: "system",
          message: "RGD Revision 1 (initial)",
          isCompleted: true,
          isCurrent: false,
          revisionNumber: 1,
        },
        {
          timestamp: "2024-01-01T00:05:00Z",
          eventType: "Ready",
          status: "Ready",
          isCompleted: true,
          isCurrent: true,
        },
      ],
    };

    setupMocks({ data: mixedTimeline });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    // All three event types should render
    expect(screen.getByText("Created")).toBeInTheDocument();
    expect(screen.getByText("Revision Changed")).toBeInTheDocument();
    // "Ready" appears in both the node label and auto-selected detail card
    expect(screen.getAllByText("Ready").length).toBeGreaterThanOrEqual(1);

    // Count should reflect all entries
    expect(screen.getByText("3 events")).toBeInTheDocument();
  });

  it("disables download button while exporting", () => {
    setupMocks({}, { isPending: true });

    render(<DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />, {
      wrapper: createWrapper(),
    });

    const downloadButton = screen.getByRole("button", { name: /download/i });
    expect(downloadButton).toBeDisabled();
  });

  // =========================================================================
  // Horizontal timeline specific tests
  // =========================================================================

  it("renders proportional connectors between nodes", () => {
    const timeline: TimelineResponse = {
      namespace: "test-ns",
      kind: "WebApp",
      name: "test-instance",
      timeline: [
        {
          timestamp: "2024-01-01T00:00:00Z",
          eventType: "Created",
          status: "Pending",
          isCompleted: true,
          isCurrent: false,
        },
        {
          timestamp: "2024-01-01T00:01:00Z",
          eventType: "Ready",
          status: "Ready",
          isCompleted: true,
          isCurrent: false,
        },
        {
          timestamp: "2024-01-01T01:00:00Z",
          eventType: "StatusChanged",
          status: "Degraded",
          isCompleted: true,
          isCurrent: true,
        },
      ],
    };

    setupMocks({ data: timeline });

    const { container } = render(
      <DeploymentTimeline namespace="test-ns" kind="WebApp" name="test-instance" />,
      { wrapper: createWrapper() }
    );

    // There should be connector divs between nodes
    const connectors = container.querySelectorAll(".bg-border.h-px");
    expect(connectors.length).toBe(2);
  });
});
