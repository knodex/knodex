// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RevisionsTab } from "./RevisionsTab";
import type { GraphRevisionList, RevisionDiff } from "@/types/rgd";

// Mock useRGDRevisions hook
const mockRevisionsData: GraphRevisionList = {
  items: [
    {
      revisionNumber: 3,
      rgdName: "my-webapp",
      namespace: "default",
      conditions: [
        { type: "GraphVerified", status: "True" },
        { type: "Ready", status: "True" },
      ],
      contentHash: "abc123def456",
      createdAt: "2026-03-28T10:00:00Z",
    },
    {
      revisionNumber: 2,
      rgdName: "my-webapp",
      namespace: "default",
      conditions: [
        { type: "GraphVerified", status: "True" },
        { type: "Ready", status: "False", reason: "ReconcileError" },
      ],
      contentHash: "xyz789",
      createdAt: "2026-03-27T10:00:00Z",
    },
    {
      revisionNumber: 1,
      rgdName: "my-webapp",
      namespace: "default",
      conditions: [
        { type: "GraphVerified", status: "True" },
        { type: "Ready", status: "True" },
      ],
      contentHash: "first111",
      createdAt: "2026-03-26T10:00:00Z",
    },
  ],
  totalCount: 3,
};

const mockDiff: RevisionDiff = {
  rgdName: "my-webapp",
  rev1: 2,
  rev2: 3,
  added: [{ path: "kind", newValue: "RGD" }],
  removed: [],
  modified: [],
  identical: false,
};

let mockIsLoading = false;
let mockData: GraphRevisionList | undefined = mockRevisionsData;
let mockError: Error | null = null;
let mockDiffIsLoading = false;
let mockDiffData: RevisionDiff | undefined = mockDiff;

vi.mock("@/hooks/useRGDs", () => ({
  useRGDRevisions: () => ({
    data: mockData,
    isLoading: mockIsLoading,
    error: mockError,
  }),
  useRGDRevisionDiff: () => ({
    data: mockDiffData,
    isLoading: mockDiffIsLoading,
    error: null,
  }),
  useRGDRevision: () => ({
    data: undefined,
    isLoading: false,
  }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

function renderRevisionsTab(props: { rgdName: string; currentRevision?: number }) {
  return render(
    <QueryClientProvider client={queryClient}>
      <RevisionsTab {...props} />
    </QueryClientProvider>
  );
}

describe("RevisionsTab", () => {
  beforeEach(() => {
    mockIsLoading = false;
    mockData = mockRevisionsData;
    mockError = null;
    mockDiffIsLoading = false;
    mockDiffData = mockDiff;
  });

  it("renders revision list with correct count", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.getByText("3 revisions")).toBeInTheDocument();
    expect(screen.getByTestId("revision-3")).toBeInTheDocument();
    expect(screen.getByTestId("revision-2")).toBeInTheDocument();
    expect(screen.getByTestId("revision-1")).toBeInTheDocument();
  });

  it("renders revision numbers", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.getByText("#3")).toBeInTheDocument();
    expect(screen.getByText("#2")).toBeInTheDocument();
    expect(screen.getByText("#1")).toBeInTheDocument();
  });

  it("renders condition badges", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    const badges = screen.getAllByText("GraphVerified");
    expect(badges.length).toBe(3);

    const readyBadges = screen.getAllByText("Ready");
    expect(readyBadges.length).toBe(3);
  });

  it("renders truncated content hash", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.getByText("abc123d")).toBeInTheDocument();
    expect(screen.getByText("xyz789")).toBeInTheDocument();
  });

  it("shows Current badge on the active revision", () => {
    renderRevisionsTab({ rgdName: "my-webapp", currentRevision: 3 });

    const currentBadge = screen.getByTestId("current-badge");
    expect(currentBadge).toBeInTheDocument();
    expect(currentBadge).toHaveTextContent("Current");
  });

  it("does not show Current badge when no currentRevision prop", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.queryByTestId("current-badge")).not.toBeInTheDocument();
  });

  it("renders empty state when no revisions", () => {
    mockData = { items: [], totalCount: 0 };

    renderRevisionsTab({ rgdName: "empty-rgd" });

    expect(screen.getByText("No revisions found")).toBeInTheDocument();
  });

  it("renders loading skeletons", () => {
    mockIsLoading = true;
    mockData = undefined;

    const { container } = renderRevisionsTab({ rgdName: "loading-rgd" });

    const skeletons = container.querySelectorAll(".animate-token-shimmer");
    expect(skeletons.length).toBe(3);
  });

  it("renders error state when API fails", () => {
    mockError = new Error("Network error");
    mockData = undefined;

    renderRevisionsTab({ rgdName: "error-rgd" });

    expect(screen.getByText("Failed to load revisions")).toBeInTheDocument();
    expect(screen.getByText("Network error")).toBeInTheDocument();
  });

  it("renders checkboxes for revision selection when multiple revisions exist", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.getByTestId("revision-select-3")).toBeInTheDocument();
    expect(screen.getByTestId("revision-select-2")).toBeInTheDocument();
    expect(screen.getByTestId("revision-select-1")).toBeInTheDocument();
  });

  it("pre-selects the latest two revisions by default", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    const checkbox3 = screen.getByTestId("revision-select-3") as HTMLInputElement;
    const checkbox2 = screen.getByTestId("revision-select-2") as HTMLInputElement;
    const checkbox1 = screen.getByTestId("revision-select-1") as HTMLInputElement;

    expect(checkbox3.checked).toBe(true);
    expect(checkbox2.checked).toBe(true);
    expect(checkbox1.checked).toBe(false);
  });

  it("shows Compare button when two revisions are selected", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    expect(screen.getByTestId("compare-button")).toBeInTheDocument();
  });

  it("does not show checkboxes when only one revision exists", () => {
    mockData = {
      items: [
        {
          revisionNumber: 1,
          rgdName: "single",
          namespace: "default",
          conditions: [],
          createdAt: "2026-03-28T10:00:00Z",
        },
      ],
      totalCount: 1,
    };

    renderRevisionsTab({ rgdName: "single" });

    expect(screen.queryByTestId("revision-select-1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("compare-button")).not.toBeInTheDocument();
  });

  it("shows diff view when Compare button is clicked", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    const compareBtn = screen.getByTestId("compare-button");
    fireEvent.click(compareBtn);

    // RevisionDiffView should be rendered
    expect(screen.getByLabelText("Close diff view")).toBeInTheDocument();
  });

  it("hides diff view when close button is clicked", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    // Open diff
    fireEvent.click(screen.getByTestId("compare-button"));
    expect(screen.getByLabelText("Close diff view")).toBeInTheDocument();

    // Close diff
    fireEvent.click(screen.getByLabelText("Close diff view"));
    expect(screen.queryByLabelText("Close diff view")).not.toBeInTheDocument();
  });

  it("toggles selection when a different checkbox is clicked", () => {
    renderRevisionsTab({ rgdName: "my-webapp" });

    // Default: rev 2 and 3 selected. Click rev 1 to swap out the farther one.
    const checkbox1 = screen.getByTestId("revision-select-1") as HTMLInputElement;
    fireEvent.click(checkbox1);

    // Rev 1 should now be selected along with rev 2 or rev 3.
    expect(checkbox1.checked).toBe(true);
  });
});
