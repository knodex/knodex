import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuditFilters } from "./AuditFilters";
import { isoToLocalDatetime } from "./utils";

// Mock useProjects to avoid real API calls
vi.mock("@/hooks/useProjects", () => ({
  useProjects: () => ({
    data: {
      items: [
        { name: "alpha", description: "Alpha project" },
        { name: "beta", description: "Beta project" },
      ],
      totalCount: 2,
    },
    isLoading: false,
  }),
}));

const defaultProps = {
  userId: "",
  action: "",
  resource: "",
  project: "",
  result: "",
  from: "",
  to: "",
  onFilterChange: vi.fn(),
  onClearFilters: vi.fn(),
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("AuditFilters", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders all filter controls", () => {
    render(<AuditFilters {...defaultProps} />, { wrapper: createWrapper() });

    expect(screen.getByText("User")).toBeInTheDocument();
    expect(screen.getByText("Action")).toBeInTheDocument();
    expect(screen.getByText("Resource")).toBeInTheDocument();
    expect(screen.getByText("Project")).toBeInTheDocument();
    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(screen.getByText("From")).toBeInTheDocument();
    expect(screen.getByText("To")).toBeInTheDocument();
  });

  it("renders project filter as a dropdown with projects from API", () => {
    render(<AuditFilters {...defaultProps} />, { wrapper: createWrapper() });

    // Project filter should show "All projects" as default trigger text
    expect(screen.getByText("All projects")).toBeInTheDocument();

    // Verify the project filter is a combobox (Select), not a text input
    const comboboxes = screen.getAllByRole("combobox");
    const projectCombobox = comboboxes.find(
      (el) => el.textContent?.includes("All projects")
    );
    expect(projectCombobox).toBeDefined();
  });

  it("does not show clear button when no filters are active", () => {
    render(<AuditFilters {...defaultProps} />, { wrapper: createWrapper() });

    expect(screen.queryByText("Clear")).not.toBeInTheDocument();
  });

  it("shows clear button when filters are active", () => {
    render(<AuditFilters {...defaultProps} userId="admin" />, { wrapper: createWrapper() });

    expect(screen.getByText("Clear")).toBeInTheDocument();
  });

  it("calls onClearFilters when clear button is clicked", async () => {
    const onClearFilters = vi.fn();
    render(
      <AuditFilters {...defaultProps} userId="admin" onClearFilters={onClearFilters} />,
      { wrapper: createWrapper() }
    );

    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    await user.click(screen.getByText("Clear"));
    expect(onClearFilters).toHaveBeenCalledTimes(1);
  });

  it("debounces userId text input (400ms)", async () => {
    const onFilterChange = vi.fn();
    render(
      <AuditFilters {...defaultProps} onFilterChange={onFilterChange} />,
      { wrapper: createWrapper() }
    );

    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const input = screen.getByPlaceholderText("Filter by user...");
    await user.type(input, "admin");

    // Should not have called onFilterChange immediately for each keystroke
    expect(onFilterChange).not.toHaveBeenCalled();

    // Advance past debounce timer
    act(() => { vi.advanceTimersByTime(500); });

    // Should have called once with final value
    expect(onFilterChange).toHaveBeenCalledTimes(1);
    expect(onFilterChange).toHaveBeenCalledWith("userId", "admin");
  });

  it("syncs local userId state when prop changes", () => {
    const wrapper = createWrapper();
    const { rerender } = render(
      <AuditFilters {...defaultProps} userId="admin" />,
      { wrapper }
    );

    const input = screen.getByPlaceholderText("Filter by user...") as HTMLInputElement;
    expect(input.value).toBe("admin");

    rerender(<AuditFilters {...defaultProps} userId="" />);
    expect(input.value).toBe("");
  });

  it("displays active filter values from props", () => {
    render(
      <AuditFilters {...defaultProps} userId="admin@test.local" />,
      { wrapper: createWrapper() }
    );

    const userInput = screen.getByPlaceholderText("Filter by user...") as HTMLInputElement;
    expect(userInput.value).toBe("admin@test.local");
  });
});

describe("isoToLocalDatetime", () => {
  it("returns empty string for invalid date", () => {
    expect(isoToLocalDatetime("garbage")).toBe("");
    expect(isoToLocalDatetime("")).toBe("");
  });

  it("converts valid ISO date to local datetime string", () => {
    const result = isoToLocalDatetime("2026-06-15T14:30:00Z");
    // Should be in YYYY-MM-DDTHH:mm format (local timezone)
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/);
  });
});
