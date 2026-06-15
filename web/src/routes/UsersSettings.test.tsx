// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import * as React from "react";
import { render, screen, fireEvent, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/components/ui/tooltip";
import { UsersSettings } from "./UsersSettings";
import * as useUsersHook from "@/hooks/useUsers";
import * as useLicenseHook from "@/hooks/useLicense";
import * as useCanIHook from "@/hooks/useCanI";
import { toast } from "sonner";
import type { User } from "@/types/user";
import type { LicenseStatus, SeatUsage } from "@/types/license";

vi.mock("@/hooks/useUsers");
vi.mock("@/hooks/useLicense");
vi.mock("@/hooks/useCanI");
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

// Mock Radix Select to avoid portal/pointer event issues in jsdom (same pattern
// as compliance/EnforcementSelector.test.tsx). The mock surfaces options as
// click targets that drive onValueChange directly.
let selectOnValueChange: ((value: string) => void) | undefined;
vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    onValueChange,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => {
    selectOnValueChange = onValueChange;
    return <div data-testid="select-root">{children}</div>;
  },
  SelectTrigger: ({
    children,
    ...props
  }: {
    children: React.ReactNode;
  } & React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button role="combobox" {...props}>
      {children}
    </button>
  ),
  SelectValue: ({ children }: { children?: React.ReactNode }) => (
    <span>{children}</span>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="select-content">{children}</div>
  ),
  SelectItem: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => (
    <div
      role="option"
      data-testid={`option-${value}`}
      onClick={() => selectOnValueChange?.(value)}
    >
      {children}
    </div>
  ),
}));

// Mock FiltersDropdown to render its children inline (no popover to open) and
// expose the active-count badge so AC #3's count can be asserted.
vi.mock("@/components/ui/filters-dropdown", () => ({
  FiltersDropdown: ({
    children,
    activeCount = 0,
  }: {
    children: React.ReactNode;
    activeCount?: number;
  }) => (
    <div data-testid="filters-dropdown">
      <span data-testid="filters-active-count">{activeCount}</span>
      {children}
    </div>
  ),
}));

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: "u1",
    email: "alice@test.local",
    displayName: "Alice Admin",
    state: "active",
    isInactive: false,
    applicationRole: "member",
    firstSeenAt: "2026-01-15T10:00:00Z",
    lastSeenAt: "2026-06-01T10:00:00Z",
    federatedIdentities: [
      {
        issuer: "https://idp.test.local",
        sub: "alice-sub",
        providerKind: "oidc",
        sourceKind: "oidc_jit",
        createdAt: "2026-01-15T10:00:00Z",
        updatedAt: "2026-06-01T10:00:00Z",
      },
    ],
    ...overrides,
  };
}

// Minimal shape of the useUsers return used by the component.
type UseUsersReturn = ReturnType<typeof useUsersHook.useUsers>;

function mockUseUsers(partial: Partial<UseUsersReturn>) {
  vi.mocked(useUsersHook.useUsers).mockReturnValue({
    data: undefined,
    isLoading: false,
    error: null,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
    ...partial,
  } as unknown as UseUsersReturn);
}

/** Render with the app-wide TooltipProvider that lives in App.tsx at runtime. */
function renderPage() {
  return render(
    <TooltipProvider>
      <UsersSettings />
    </TooltipProvider>,
  );
}

/** Convenience: mock a roster of the given users. */
function mockRoster(users: User[]) {
  mockUseUsers({
    data: {
      pages: [{ users }],
      pageParams: [undefined],
    } as unknown as UseUsersReturn["data"],
  });
}

// Shared reclaim mutation mock — resolves by default (a real reclaim, or a 404
// the hook has already swallowed into a success). Individual tests override it.
const reclaimMutateAsync = vi.fn();

function mockReclaim({ isPending = false }: { isPending?: boolean } = {}) {
  vi.mocked(useUsersHook.useReclaimUser).mockReturnValue({
    mutateAsync: reclaimMutateAsync,
    isPending,
  } as unknown as ReturnType<typeof useUsersHook.useReclaimUser>);
}

function makeSeats(overrides: Partial<SeatUsage> = {}): SeatUsage {
  return {
    used: 3,
    allowed: 10,
    windowDays: 30,
    percent: 0.3,
    threshold: "ok",
    lastUpdated: "2026-06-01T10:00:00Z",
    advisoryOnly: false,
    ...overrides,
  };
}

/** Mock the license query. Pass `undefined` for the OSS (no-seats) case. */
function mockLicense(seats: SeatUsage | undefined, maxUsers = 10) {
  vi.mocked(useLicenseHook.useLicenseStatus).mockReturnValue({
    data: seats
      ? ({
          licensed: true,
          enterprise: true,
          status: "valid",
          message: "",
          license: { maxUsers } as LicenseStatus["license"],
          seats,
        } as LicenseStatus)
      : undefined,
  } as unknown as ReturnType<typeof useLicenseHook.useLicenseStatus>);
}

/** Mock the operator can-i gate. */
function mockCanI({
  allowed = true,
  isError = false,
}: { allowed?: boolean | undefined; isError?: boolean } = {}) {
  vi.mocked(useCanIHook.useCanI).mockReturnValue({
    allowed,
    isLoading: false,
    isError,
  });
}

describe("UsersSettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    selectOnValueChange = undefined;
    reclaimMutateAsync.mockResolvedValue({
      id: "u1",
      state: "removed",
      note: "Seat reclaimed. Permanent exclusion requires IdP-side revocation.",
    });
    mockReclaim();
    mockLicense(undefined); // default: OSS / no seat widget unless a test opts in
    mockCanI({ allowed: true });
  });

  it("renders a roster row with state badge, issuer chip, and timestamps", () => {
    mockRoster([makeUser()]);

    renderPage();

    // Email + display name
    expect(screen.getByText("alice@test.local")).toBeInTheDocument();
    expect(screen.getByText("Alice Admin")).toBeInTheDocument();
    // State badge
    expect(screen.getByTestId("user-state-badge")).toHaveTextContent("Active");
    // Issuer chip (de-duped)
    const chips = screen.getAllByTestId("user-issuer-chip");
    expect(chips).toHaveLength(1);
    expect(chips[0]).toHaveTextContent("https://idp.test.local");
    // ListFooter active breakdown
    const footer = screen.getByTestId("users-list-footer");
    expect(footer).toHaveTextContent("users");
    expect(footer).toHaveTextContent("active");
  });

  it("de-duplicates repeated issuers into a single chip", () => {
    const user = makeUser({
      federatedIdentities: [
        {
          issuer: "https://idp.test.local",
          sub: "a",
          providerKind: "oidc",
          sourceKind: "oidc_jit",
          createdAt: "2026-01-15T10:00:00Z",
          updatedAt: "2026-06-01T10:00:00Z",
        },
        {
          issuer: "https://idp.test.local",
          sub: "b",
          providerKind: "oidc",
          sourceKind: "oidc_jit",
          createdAt: "2026-01-15T10:00:00Z",
          updatedAt: "2026-06-01T10:00:00Z",
        },
      ],
    });
    mockRoster([user]);

    renderPage();

    expect(screen.getAllByTestId("user-issuer-chip")).toHaveLength(1);
  });

  it("renders a removed state badge", () => {
    mockRoster([makeUser({ state: "removed" })]);

    renderPage();

    expect(screen.getByTestId("user-state-badge")).toHaveTextContent("Removed");
  });

  it("renders the Access Denied state on a 403", () => {
    mockUseUsers({
      error: Object.assign(new Error("Forbidden"), {
        response: { status: 403 },
      }) as unknown as UseUsersReturn["error"],
    });

    renderPage();

    expect(screen.getByTestId("users-access-denied")).toBeInTheDocument();
    expect(screen.getByText("Access Denied")).toBeInTheDocument();
    // Table is NOT rendered.
    expect(screen.queryByTestId("users-settings")).not.toBeInTheDocument();
  });

  it("renders the empty state on an empty roster", () => {
    mockRoster([]);

    renderPage();

    expect(screen.getByTestId("users-empty-state")).toBeInTheDocument();
    expect(screen.queryByTestId("user-state-badge")).not.toBeInTheDocument();
  });

  it("renders a loading skeleton while fetching", () => {
    mockUseUsers({ isLoading: true });

    renderPage();

    expect(screen.getByTestId("users-loading")).toBeInTheDocument();
  });

  it("shows the Load more control when another page is available", () => {
    mockUseUsers({
      data: {
        pages: [{ users: [makeUser()], nextPageToken: "cursor-2" }],
        pageParams: [undefined],
      } as unknown as UseUsersReturn["data"],
      hasNextPage: true,
    });

    renderPage();

    expect(screen.getByTestId("users-load-more")).toBeInTheDocument();
  });

  it("invokes fetchNextPage when Load more is clicked", () => {
    const fetchNextPage = vi.fn();
    mockUseUsers({
      data: {
        pages: [{ users: [makeUser()], nextPageToken: "cursor-2" }],
        pageParams: [undefined],
      } as unknown as UseUsersReturn["data"],
      hasNextPage: true,
      fetchNextPage: fetchNextPage as unknown as UseUsersReturn["fetchNextPage"],
    });

    renderPage();

    fireEvent.click(screen.getByTestId("users-load-more"));
    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });

  // ─── Story 16.3: inactive badge ───

  it("renders the inactive badge only for isInactive rows (AC #1)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "active@test.local", isInactive: false }),
      makeUser({ id: "u2", email: "idle@test.local", isInactive: true }),
    ]);

    renderPage();

    // Exactly one inactive badge, on the idle row only.
    const badges = screen.getAllByTestId("user-inactive-badge");
    expect(badges).toHaveLength(1);
    const idleRow = screen.getByTestId("user-row-u2");
    expect(within(idleRow).getByTestId("user-inactive-badge")).toBeInTheDocument();
    const activeRow = screen.getByTestId("user-row-u1");
    expect(
      within(activeRow).queryByTestId("user-inactive-badge"),
    ).not.toBeInTheDocument();
  });

  it("inactive badge tooltip states it does not affect billing (AC #1)", async () => {
    const user = userEvent.setup();
    mockRoster([makeUser({ isInactive: true })]);

    renderPage();

    await user.hover(screen.getByTestId("user-inactive-badge"));

    // Radix renders the content on hover; assert the not-billing wording.
    const tip = await screen.findAllByText(/does not affect billing/i);
    expect(tip.length).toBeGreaterThan(0);
  });

  // ─── Story 17.3: read-only application role badge (Path A) ───

  it("renders serveradmin vs member application-role badges (AC #4)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "admin@test.local", applicationRole: "serveradmin" }),
      makeUser({ id: "u2", email: "member@test.local", applicationRole: "member" }),
    ]);

    renderPage();

    const adminRow = screen.getByTestId("user-row-u1");
    expect(
      within(adminRow).getByTestId("user-app-role-badge"),
    ).toHaveTextContent("Server admin");

    const memberRow = screen.getByTestId("user-row-u2");
    expect(
      within(memberRow).getByTestId("user-app-role-badge"),
    ).toHaveTextContent("Member");
  });

  it("shows NO control to change a user's application role — read-only (AC #4)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "admin@test.local", applicationRole: "serveradmin" }),
    ]);

    renderPage();

    const row = screen.getByTestId("user-row-u1");
    // The cell carries only the badge — no select/combobox/edit affordance.
    expect(within(row).getByTestId("user-app-role-badge")).toBeInTheDocument();
    expect(within(row).queryByRole("combobox")).not.toBeInTheDocument();
    expect(
      within(row).queryByTestId("user-app-role-select"),
    ).not.toBeInTheDocument();
    expect(
      within(row).queryByRole("button", { name: /role/i }),
    ).not.toBeInTheDocument();
  });

  // ─── Story 16.3: search ───

  it("search narrows rows by email/displayName (AC #2)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", displayName: "Alice Admin" }),
      makeUser({ id: "u2", email: "bob@test.local", displayName: "Bob Builder" }),
    ]);

    renderPage();

    expect(screen.getByTestId("user-row-u1")).toBeInTheDocument();
    expect(screen.getByTestId("user-row-u2")).toBeInTheDocument();

    await user.type(screen.getByTestId("users-search"), "builder");

    expect(screen.queryByTestId("user-row-u1")).not.toBeInTheDocument();
    expect(screen.getByTestId("user-row-u2")).toBeInTheDocument();

    // Footer reflects the filtered set (1 user, the total leads the summary).
    const footer = screen.getByTestId("users-list-footer");
    expect(footer.firstElementChild).toHaveTextContent("1 users");
  });

  it("clearing the search restores the full loaded set (AC #2)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", displayName: "Alice Admin" }),
      makeUser({ id: "u2", email: "bob@test.local", displayName: "Bob Builder" }),
    ]);

    renderPage();

    await user.type(screen.getByTestId("users-search"), "alice");
    expect(screen.queryByTestId("user-row-u2")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("users-search-clear"));

    expect(screen.getByTestId("user-row-u1")).toBeInTheDocument();
    expect(screen.getByTestId("user-row-u2")).toBeInTheDocument();
  });

  it("search matches email OR displayName independently, not the concatenation (AC #2)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", displayName: "Alice Admin" }),
      makeUser({ id: "u2", email: "bob@test.local", displayName: "Bob Builder" }),
    ]);

    renderPage();

    // A query that only matches when email+displayName are concatenated with a
    // space ("...test.local Alice...") must NOT match either field on its own.
    await user.type(screen.getByTestId("users-search"), "local alice");

    expect(screen.getByTestId("users-no-matches")).toBeInTheDocument();
    expect(screen.queryByTestId("user-row-u1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("user-row-u2")).not.toBeInTheDocument();
  });

  // ─── Story 16.3: filters ───

  it("state filter narrows rows (AC #3)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "active@test.local", state: "active" }),
      makeUser({ id: "u2", email: "removed@test.local", state: "removed" }),
    ]);

    renderPage();

    // Select "Removed" via the mocked Select option.
    fireEvent.click(screen.getByTestId("option-removed"));

    expect(screen.queryByTestId("user-row-u1")).not.toBeInTheDocument();
    expect(screen.getByTestId("user-row-u2")).toBeInTheDocument();

    // FiltersDropdown active-count reflects one engaged filter.
    expect(screen.getByTestId("filters-active-count")).toHaveTextContent("1");

    // Footer counts the filtered set: 1 user, 0 active.
    const footer = screen.getByTestId("users-list-footer");
    expect(within(footer).getAllByText("0").length).toBeGreaterThan(0);
  });

  it("inactive-only filter narrows rows (AC #3)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "active@test.local", isInactive: false }),
      makeUser({ id: "u2", email: "idle@test.local", isInactive: true }),
    ]);

    renderPage();

    fireEvent.click(screen.getByTestId("users-inactive-filter"));

    expect(screen.queryByTestId("user-row-u1")).not.toBeInTheDocument();
    expect(screen.getByTestId("user-row-u2")).toBeInTheDocument();
    expect(screen.getByTestId("filters-active-count")).toHaveTextContent("1");
  });

  // ─── Story 16.3: footer reflects the filtered set (AC #4, headline) ───

  it("ListFooter total equals the filtered count, not the loaded count (AC #4)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", state: "active" }),
      makeUser({ id: "u2", email: "bob@test.local", state: "active" }),
      makeUser({ id: "u3", email: "carol@test.local", state: "active" }),
    ]);

    renderPage();

    // Unfiltered: footer total is 3.
    let footer = screen.getByTestId("users-list-footer");
    expect(footer.firstElementChild).toHaveTextContent("3 users");

    // Search to a single match — footer total drops to 1 (the filtered count),
    // NOT the 3 rows that remain loaded.
    await user.type(screen.getByTestId("users-search"), "carol");

    footer = screen.getByTestId("users-list-footer");
    expect(footer.firstElementChild).toHaveTextContent("1 users");
    expect(footer).not.toHaveTextContent("3 users");
  });

  // ─── Story 16.3: no-matches empty state (AC #4) ───

  it("renders the no-matches state on a zero-result filter, distinct from empty roster (AC #4)", async () => {
    const user = userEvent.setup();
    mockRoster([makeUser({ id: "u1", email: "alice@test.local" })]);

    renderPage();

    await user.type(screen.getByTestId("users-search"), "zzz-nobody");

    expect(screen.getByTestId("users-no-matches")).toBeInTheDocument();
    // It is NOT the genuine "No users yet" roster-empty state.
    expect(screen.queryByTestId("users-empty-state")).not.toBeInTheDocument();
    // No rows and no footer when nothing matches.
    expect(screen.queryByTestId("user-row-u1")).not.toBeInTheDocument();
    expect(screen.queryByTestId("users-list-footer")).not.toBeInTheDocument();
  });

  // ─── Story 16.2: seat-usage widget ───

  it("renders the seat-usage widget in the header on EE (seats present) (AC #1)", () => {
    mockLicense(makeSeats({ used: 3, allowed: 10 }), 10);
    mockRoster([makeUser()]);

    renderPage();

    expect(screen.getByTestId("users-seat-usage-header")).toBeInTheDocument();
    expect(screen.getByTestId("users-seat-usage")).toHaveTextContent("3 / 10");
  });

  it("does NOT render the seat widget on OSS (seats undefined); roster still renders (AC #1)", () => {
    mockLicense(undefined);
    mockRoster([makeUser()]);

    renderPage();

    expect(
      screen.queryByTestId("users-seat-usage-header"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("users-seat-usage")).not.toBeInTheDocument();
    expect(screen.getByText("alice@test.local")).toBeInTheDocument();
  });

  // ─── Story 16.2: reclaim action visibility ───

  it("shows the Reclaim action on active rows only, never on removed rows (AC #2)", () => {
    mockRoster([
      makeUser({ id: "u1", email: "active@test.local", state: "active" }),
      makeUser({ id: "u2", email: "removed@test.local", state: "removed" }),
    ]);

    renderPage();

    expect(screen.getByTestId("reclaim-seat-u1")).toBeInTheDocument();
    expect(screen.queryByTestId("reclaim-seat-u2")).not.toBeInTheDocument();
  });

  it("hides the Reclaim action when the operator lacks settings/* update (AC #2)", () => {
    mockCanI({ allowed: false });
    mockRoster([makeUser({ id: "u1", state: "active" })]);

    renderPage();

    expect(screen.queryByTestId("reclaim-seat-u1")).not.toBeInTheDocument();
  });

  it("shows the Reclaim action when the can-i check errors — server still enforces (AC #2)", () => {
    mockCanI({ allowed: undefined, isError: true });
    mockRoster([makeUser({ id: "u1", state: "active" })]);

    renderPage();

    expect(screen.getByTestId("reclaim-seat-u1")).toBeInTheDocument();
  });

  // ─── Story 16.2: confirm dialog copy (the headline constraint) ───

  it("dialog carries the verbatim revocation note + resurrection framing and NO hard-delete wording (AC #3, #6)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", state: "active" }),
    ]);

    renderPage();
    await user.click(screen.getByTestId("reclaim-seat-u1"));

    const dialog = await screen.findByTestId("reclaim-seat-dialog");
    expect(dialog).toHaveTextContent(
      "Permanent exclusion requires IdP-side revocation",
    );
    expect(dialog).toHaveTextContent(/reappears on their next SSO login/i);

    const text = (dialog.textContent ?? "").toLowerCase();
    expect(text).not.toContain("delete");
    expect(text).not.toContain("permanently");
    expect(text).not.toContain("cannot be undone");
  });

  it("confirming calls the reclaim mutation with the user id and shows a reclaim (not delete) toast (AC #4)", async () => {
    const user = userEvent.setup();
    mockRoster([
      makeUser({ id: "u1", email: "alice@test.local", state: "active" }),
    ]);

    renderPage();
    await user.click(screen.getByTestId("reclaim-seat-u1"));
    await user.click(await screen.findByTestId("reclaim-seat-confirm"));

    expect(reclaimMutateAsync).toHaveBeenCalledWith("u1");
    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    const msg = String(
      vi.mocked(toast.success).mock.calls[0][0],
    ).toLowerCase();
    expect(msg).toContain("reclaimed");
    expect(msg).not.toContain("delete");
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("a 404 (already removed) resolves without an error toast (AC #4)", async () => {
    // The hook converts a 404 into a resolved success, so from the component's
    // vantage mutateAsync RESOLVES — assert no error toast fires.
    const user = userEvent.setup();
    reclaimMutateAsync.mockResolvedValue({ id: "u1", state: "removed", note: "" });
    mockRoster([makeUser({ id: "u1", state: "active" })]);

    renderPage();
    await user.click(screen.getByTestId("reclaim-seat-u1"));
    await user.click(await screen.findByTestId("reclaim-seat-confirm"));

    await waitFor(() => expect(toast.success).toHaveBeenCalled());
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("surfaces a single error toast on a non-404 failure (AC #4)", async () => {
    const user = userEvent.setup();
    reclaimMutateAsync.mockRejectedValue(
      Object.assign(new Error("server boom"), {
        response: { status: 500, data: { message: "server boom" } },
      }),
    );
    mockRoster([makeUser({ id: "u1", state: "active" })]);

    renderPage();
    await user.click(screen.getByTestId("reclaim-seat-u1"));
    await user.click(await screen.findByTestId("reclaim-seat-confirm"));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("server boom"));
    expect(toast.success).not.toHaveBeenCalled();
  });
});
