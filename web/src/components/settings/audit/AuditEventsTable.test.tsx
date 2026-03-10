// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AuditEventsTable } from "./AuditEventsTable";
import type { AuditEvent } from "@/types/audit";

const mockEvent: AuditEvent = {
  id: "01HTEST123",
  timestamp: new Date(Date.now() - 5 * 60 * 1000).toISOString(), // 5 min ago
  userId: "user-1",
  userEmail: "admin@test.local",
  sourceIP: "10.0.0.1",
  action: "create",
  resource: "projects",
  name: "alpha",
  project: "default",
  requestId: "req-abc",
  result: "success",
};

const defaultProps = {
  events: [] as AuditEvent[],
  total: 0,
  page: 1,
  pageSize: 50,
  isLoading: false,
  onPageChange: vi.fn(),
  onPageSizeChange: vi.fn(),
  onSortChange: vi.fn(),
  onRowClick: vi.fn(),
};

describe("AuditEventsTable", () => {
  it("renders empty state when no events and not loading", () => {
    render(<AuditEventsTable {...defaultProps} />);

    expect(screen.getByText("No audit events found")).toBeInTheDocument();
    expect(screen.getByText(/Try adjusting your filters/)).toBeInTheDocument();
  });

  it("renders loading skeleton when loading with no events", () => {
    const { container } = render(
      <AuditEventsTable {...defaultProps} isLoading={true} />
    );

    // Should show skeleton rows, not the table
    expect(screen.queryByText("No audit events found")).not.toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    // Skeletons are rendered as divs with animation classes
    const skeletons = container.querySelectorAll("[class*='animate-pulse']");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("renders event rows with correct data", () => {
    render(
      <AuditEventsTable {...defaultProps} events={[mockEvent]} total={1} />
    );

    expect(screen.getByText("admin@test.local")).toBeInTheDocument();
    expect(screen.getByText("create")).toBeInTheDocument();
    expect(screen.getByText("projects")).toBeInTheDocument();
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
    expect(screen.getByText("success")).toBeInTheDocument();
  });

  it("renders sortable column headers", () => {
    render(
      <AuditEventsTable {...defaultProps} events={[mockEvent]} total={1} />
    );

    expect(screen.getByText("Time")).toBeInTheDocument();
    expect(screen.getByText("User")).toBeInTheDocument();
    expect(screen.getByText("Action")).toBeInTheDocument();
    expect(screen.getByText("Resource")).toBeInTheDocument();
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByText("Result")).toBeInTheDocument();
  });

  it("calls onSortChange when column header is clicked", async () => {
    const onSortChange = vi.fn();
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={1}
        onSortChange={onSortChange}
      />
    );

    const user = userEvent.setup();
    await user.click(screen.getByText("Time"));
    expect(onSortChange).toHaveBeenCalledWith("timestamp", "asc");
  });

  it("toggles sort order when same column clicked again", async () => {
    const onSortChange = vi.fn();
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={1}
        sortBy="timestamp"
        sortOrder="asc"
        onSortChange={onSortChange}
      />
    );

    const user = userEvent.setup();
    await user.click(screen.getByText("Time"));
    expect(onSortChange).toHaveBeenCalledWith("timestamp", "desc");
  });

  it("clears sort (unsort) when desc column clicked again (3-state cycle)", async () => {
    const onSortChange = vi.fn();
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={1}
        sortBy="timestamp"
        sortOrder="desc"
        onSortChange={onSortChange}
      />
    );

    const user = userEvent.setup();
    await user.click(screen.getByText("Time"));
    expect(onSortChange).toHaveBeenCalledWith(undefined, undefined);
  });

  it("calls onRowClick when event row is clicked", async () => {
    const onRowClick = vi.fn();
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={1}
        onRowClick={onRowClick}
      />
    );

    const user = userEvent.setup();
    await user.click(screen.getByText("admin@test.local"));
    expect(onRowClick).toHaveBeenCalledWith(mockEvent);
  });

  it("renders pagination info with correct totals", () => {
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={150}
        page={2}
        pageSize={50}
      />
    );

    expect(screen.getByText("150 events")).toBeInTheDocument();
    expect(screen.getByText("Page 2 of 3")).toBeInTheDocument();
  });

  it("disables previous button on first page", () => {
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={100}
        page={1}
      />
    );

    expect(screen.getByLabelText("Previous page")).toBeDisabled();
    expect(screen.getByLabelText("Next page")).not.toBeDisabled();
  });

  it("disables next button on last page", () => {
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[mockEvent]}
        total={50}
        page={1}
        pageSize={50}
      />
    );

    expect(screen.getByLabelText("Next page")).toBeDisabled();
  });

  it("shows em dash for missing project", () => {
    const noProjectEvent = { ...mockEvent, project: undefined };
    render(
      <AuditEventsTable
        {...defaultProps}
        events={[noProjectEvent]}
        total={1}
      />
    );

    expect(screen.getByText("\u2014")).toBeInTheDocument();
  });
});
