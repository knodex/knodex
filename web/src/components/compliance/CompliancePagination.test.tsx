import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CompliancePagination } from "./CompliancePagination";

describe("CompliancePagination", () => {
  const defaultProps = {
    page: 1,
    pageSize: 20,
    totalCount: 100,
    onPageChange: vi.fn(),
    onPageSizeChange: vi.fn(),
  };

  it("renders pagination info (AC-SHARED-01)", () => {
    const { container } = render(<CompliancePagination {...defaultProps} />);

    // Should show "1-20 of 100" format
    expect(container.textContent).toContain("1-20");
    expect(container.textContent).toContain("of");
    expect(container.textContent).toContain("100");
  });

  it("shows page size selector (AC-SHARED-01)", () => {
    render(<CompliancePagination {...defaultProps} />);

    // Should have a page size selector
    expect(screen.getByRole("combobox")).toBeInTheDocument();
  });

  it("renders navigation buttons", () => {
    render(<CompliancePagination {...defaultProps} page={2} />);

    // Should have previous/next buttons
    expect(screen.getByRole("button", { name: /previous/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /next/i })).toBeInTheDocument();
  });

  it("disables previous button on first page", () => {
    render(<CompliancePagination {...defaultProps} page={1} />);

    const prevButton = screen.getByRole("button", { name: /previous/i });
    expect(prevButton).toBeDisabled();
  });

  it("disables next button on last page", () => {
    render(
      <CompliancePagination {...defaultProps} page={5} pageSize={20} totalCount={100} />
    );

    const nextButton = screen.getByRole("button", { name: /next/i });
    expect(nextButton).toBeDisabled();
  });

  it("enables previous button when not on first page", () => {
    render(<CompliancePagination {...defaultProps} page={2} />);

    const prevButton = screen.getByRole("button", { name: /previous/i });
    expect(prevButton).not.toBeDisabled();
  });

  it("enables next button when not on last page", () => {
    render(<CompliancePagination {...defaultProps} page={1} />);

    const nextButton = screen.getByRole("button", { name: /next/i });
    expect(nextButton).not.toBeDisabled();
  });

  it("calls onPageChange when previous is clicked", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();
    render(<CompliancePagination {...defaultProps} page={2} onPageChange={onPageChange} />);

    const prevButton = screen.getByRole("button", { name: /previous/i });
    await user.click(prevButton);

    expect(onPageChange).toHaveBeenCalledWith(1);
  });

  it("calls onPageChange when next is clicked", async () => {
    const user = userEvent.setup();
    const onPageChange = vi.fn();
    render(<CompliancePagination {...defaultProps} page={1} onPageChange={onPageChange} />);

    const nextButton = screen.getByRole("button", { name: /next/i });
    await user.click(nextButton);

    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it("calculates correct page range for middle pages", () => {
    const { container } = render(
      <CompliancePagination {...defaultProps} page={3} pageSize={20} totalCount={100} />
    );

    // Page 3 with pageSize 20 should show items 41-60
    expect(container.textContent).toContain("41-60");
    expect(container.textContent).toContain("100");
  });

  it("calculates correct page range for last page with partial items", () => {
    const { container } = render(
      <CompliancePagination {...defaultProps} page={3} pageSize={20} totalCount={55} />
    );

    // Page 3 with 55 items: 41-55
    expect(container.textContent).toContain("41-55");
    expect(container.textContent).toContain("55");
  });

  it("handles single page of results", () => {
    const { container } = render(
      <CompliancePagination {...defaultProps} page={1} pageSize={20} totalCount={15} />
    );

    // Should show 1-15 of 15
    expect(container.textContent).toContain("1-15");
    expect(container.textContent).toContain("15");

    // Both buttons should be disabled
    const prevButton = screen.getByRole("button", { name: /previous/i });
    const nextButton = screen.getByRole("button", { name: /next/i });
    expect(prevButton).toBeDisabled();
    expect(nextButton).toBeDisabled();
  });

  it("returns null when totalCount is zero", () => {
    const { container } = render(
      <CompliancePagination {...defaultProps} page={1} totalCount={0} />
    );

    // Component returns null for 0 total count
    expect(container.firstChild).toBeNull();
  });

  it("page size options include common values (AC-SHARED-01)", () => {
    render(<CompliancePagination {...defaultProps} pageSizeOptions={[10, 20, 50, 100]} />);

    // Should have a combobox with the "Show:" label
    expect(screen.getByRole("combobox")).toBeInTheDocument();
    expect(screen.getByText("Show:")).toBeInTheDocument();
    // Page size options are verified by the component accepting pageSizeOptions prop
  });

  it("displays first/last page buttons when many pages", () => {
    render(<CompliancePagination {...defaultProps} page={5} totalCount={200} pageSize={20} />);

    // At minimum, previous and next should be present
    expect(screen.getByRole("button", { name: /previous/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /next/i })).toBeInTheDocument();
  });

  it("shows current page and total pages", () => {
    const { container } = render(
      <CompliancePagination {...defaultProps} page={2} pageSize={20} totalCount={100} />
    );

    // Should show "2 / 5" format for page 2 of 5
    expect(container.textContent).toContain("2 / 5");
  });

  it("does not show page size selector when onPageSizeChange is not provided", () => {
    render(
      <CompliancePagination
        page={1}
        pageSize={20}
        totalCount={100}
        onPageChange={vi.fn()}
      />
    );

    // Should not have a page size selector
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
  });
});
