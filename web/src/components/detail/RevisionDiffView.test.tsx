// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RevisionDiffView } from "./RevisionDiffView";
import type { GraphRevision, RevisionDiff } from "@/types/rgd";

const mockDiff: RevisionDiff = {
  rgdName: "my-webapp",
  rev1: 1,
  rev2: 2,
  added: [{ path: "kind", newValue: "RGD" }],
  removed: [],
  modified: [{ path: "apiVersion", oldValue: "v1alpha1", newValue: "v1beta1" }],
  identical: false,
};

const rev1: GraphRevision = {
  revisionNumber: 1,
  rgdName: "my-webapp",
  namespace: "default",
  conditions: [],
  createdAt: "2026-03-27T10:00:00Z",
  snapshot: { apiVersion: "v1alpha1" },
};

const rev2: GraphRevision = {
  revisionNumber: 2,
  rgdName: "my-webapp",
  namespace: "default",
  conditions: [],
  createdAt: "2026-03-28T10:00:00Z",
  snapshot: { apiVersion: "v1beta1", kind: "RGD" },
};

let mockDiffData: RevisionDiff | undefined = mockDiff;
let mockDiffIsLoading = false;
let mockDiffError: Error | null = null;
let mockRevisionData: Record<number, GraphRevision | undefined> = { 1: rev1, 2: rev2 };
let mockRevisionIsLoading = false;

vi.mock("@/hooks/useRGDs", () => ({
  useRGDRevisionDiff: () => ({
    data: mockDiffData,
    isLoading: mockDiffIsLoading,
    error: mockDiffError,
  }),
  useRGDRevision: (_rgdName: string, revision: number | null) => ({
    data: revision !== null ? mockRevisionData[revision] : undefined,
    isLoading: mockRevisionIsLoading,
  }),
}));

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

function renderDiffView(
  overrides: Partial<{
    rev1: number;
    rev2: number;
  }> = {}
) {
  const onClose = vi.fn();
  const result = render(
    <QueryClientProvider client={queryClient}>
      <RevisionDiffView
        rgdName="my-webapp"
        rev1={overrides.rev1 ?? 1}
        rev2={overrides.rev2 ?? 2}
        onClose={onClose}
      />
    </QueryClientProvider>
  );
  return { ...result, onClose };
}

describe("RevisionDiffView", () => {
  beforeEach(() => {
    mockDiffData = mockDiff;
    mockDiffIsLoading = false;
    mockDiffError = null;
    mockRevisionData = { 1: rev1, 2: rev2 };
    mockRevisionIsLoading = false;
  });

  it("renders header with revision numbers", () => {
    renderDiffView();
    expect(screen.getByText("Rev #1 vs Rev #2")).toBeInTheDocument();
  });

  it("renders old and new YAML panel headers", () => {
    renderDiffView();
    expect(screen.getByText("Rev #1 (old)")).toBeInTheDocument();
    expect(screen.getByText("Rev #2 (new)")).toBeInTheDocument();
  });

  it("shows added field count from diff API", () => {
    renderDiffView();
    expect(screen.getByText("1 added")).toBeInTheDocument();
  });

  it("shows modified field count from diff API", () => {
    renderDiffView();
    expect(screen.getByText("1 modified")).toBeInTheDocument();
  });

  it("shows 'No differences' when diff is identical", () => {
    mockDiffData = {
      ...mockDiff,
      added: [],
      removed: [],
      modified: [],
      identical: true,
    };
    renderDiffView();
    expect(screen.getByText("No differences")).toBeInTheDocument();
  });

  it("calls onClose when close button is clicked", () => {
    const { onClose } = renderDiffView();
    fireEvent.click(screen.getByLabelText("Close diff view"));
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("renders YAML content from snapshot", () => {
    renderDiffView();
    // js-yaml serializes the snapshot — apiVersion should appear
    expect(screen.getAllByText(/apiVersion/).length).toBeGreaterThan(0);
  });

  it("shows loading indicator when diff is loading", () => {
    mockDiffIsLoading = true;
    mockDiffData = undefined;
    const { container } = renderDiffView();
    // Spinner is present (RefreshCw in header)
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("shows error message when diff API fails", () => {
    mockDiffError = new Error("server error");
    mockDiffData = undefined;
    renderDiffView();
    expect(screen.getByText(/Failed to load diff/)).toBeInTheDocument();
    expect(screen.getByText(/server error/)).toBeInTheDocument();
  });

  it("renders skeleton when loading and no snapshots available", () => {
    mockDiffIsLoading = true;
    mockDiffData = undefined;
    mockRevisionData = {};
    mockRevisionIsLoading = true;
    const { container } = renderDiffView();
    expect(container.querySelector(".animate-token-shimmer")).toBeInTheDocument();
  });
});
