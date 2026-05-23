// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SidebarNav } from "./Sidebar";

// Mock hooks consumed by SidebarNav.
vi.mock("@/hooks/useRGDs", () => ({
  useRGDList: () => ({ data: undefined }),
}));

vi.mock("@/hooks/useCompliance", () => ({
  useViolationCount: () => ({ data: 0 }),
  isEnterprise: () => false,
}));

vi.mock("@/hooks/useCategories", () => ({
  useCategoriesEnabled: () => ({ enabled: false, isLoading: false, categories: [] }),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: () => ({ allowed: false }),
}));

vi.mock("@/lib/route-preloads", () => ({
  routePreloads: {},
}));

vi.mock("@/lib/icons", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/icons")>();
  return {
    ...actual,
    getLucideIcon: () => () => null,
  };
});

const mockLogout = vi.fn();
const mockUser = { email: "alice@example.com" };
const mockUseAuth = vi.fn(() => ({
  user: mockUser as { email: string } | null,
  isAuthenticated: true,
  login: vi.fn(),
  logout: mockLogout,
}));

vi.mock("@/hooks/useAuth", () => ({
  useAuth: () => mockUseAuth(),
}));

// react-router-dom's useNavigate — preserve other exports.
const mockNavigate = vi.fn();
vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function renderSidebar(opts: {
  onNavItemClick?: () => void;
  onToggleCollapse?: () => void;
  isCollapsed?: boolean;
} = {}) {
  return render(
    <MemoryRouter>
      <SidebarNav
        onNavItemClick={opts.onNavItemClick}
        onToggleCollapse={opts.onToggleCollapse}
        isCollapsed={opts.isCollapsed}
      />
    </MemoryRouter>
  );
}

describe("SidebarNav — UserMenu", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("renders avatar trigger with the user's display name", () => {
    renderSidebar();

    const trigger = screen.getByTestId("user-menu-trigger");
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveTextContent("alice");
  });

  it("renders a skeleton placeholder when user is null (no orphan border)", () => {
    mockUseAuth.mockReturnValue({
      user: null,
      isAuthenticated: false,
      login: vi.fn(),
      logout: mockLogout,
    });

    renderSidebar();

    expect(screen.queryByTestId("user-menu-trigger")).not.toBeInTheDocument();
    // Skeleton fallback should occupy the slot
    expect(document.querySelector('[aria-busy="true"]')).toBeInTheDocument();
  });

  it("opens dropdown and exposes all four entries (Profile/Documentation/Settings/Logout)", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByTestId("user-menu-trigger"));

    expect(screen.getByTestId("user-menu-profile")).toBeInTheDocument();
    expect(screen.getByTestId("user-menu-documentation")).toBeInTheDocument();
    expect(screen.getByTestId("user-menu-settings")).toBeInTheDocument();
    expect(screen.getByTestId("user-menu-logout")).toBeInTheDocument();
  });

  it("Profile entry navigates to /user-info", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByTestId("user-menu-trigger"));
    await user.click(screen.getByTestId("user-menu-profile"));

    expect(mockNavigate).toHaveBeenCalledWith("/user-info");
  });

  it("Settings entry navigates to /settings", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByTestId("user-menu-trigger"));
    await user.click(screen.getByTestId("user-menu-settings"));

    expect(mockNavigate).toHaveBeenCalledWith("/settings");
  });

  it("Logout entry calls logout() and does NOT call navigate (auth guard handles redirect)", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByTestId("user-menu-trigger"));
    await user.click(screen.getByTestId("user-menu-logout"));

    expect(mockLogout).toHaveBeenCalledTimes(1);
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("Documentation entry points to the external docs URL with target=_blank and rel=noopener", async () => {
    const user = userEvent.setup();
    renderSidebar();

    await user.click(screen.getByTestId("user-menu-trigger"));
    // asChild renders the <a> directly as the menu item, so testid sits on the <a>.
    const docs = screen.getByTestId("user-menu-documentation");

    expect(docs.tagName).toBe("A");
    expect(docs).toHaveAttribute("href", "https://knodex.io/docs");
    expect(docs).toHaveAttribute("target", "_blank");
    expect(docs).toHaveAttribute("rel", expect.stringContaining("noopener"));
  });

  it("forwards onNavItemClick when a menu entry closes the mobile drawer", async () => {
    const user = userEvent.setup();
    const onNavItemClick = vi.fn();
    renderSidebar({ onNavItemClick });

    await user.click(screen.getByTestId("user-menu-trigger"));
    await user.click(screen.getByTestId("user-menu-settings"));

    expect(onNavItemClick).toHaveBeenCalled();
  });
});

describe("SidebarNav — collapse trigger", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("does not render the collapse button when onToggleCollapse is omitted", () => {
    renderSidebar();
    expect(screen.queryByTestId("sidebar-collapse-trigger")).not.toBeInTheDocument();
  });

  it("renders the collapse button next to the Knodex logo when expanded", () => {
    renderSidebar({ onToggleCollapse: vi.fn() });

    const btn = screen.getByTestId("sidebar-collapse-trigger");
    expect(btn).toBeInTheDocument();
    expect(btn).toHaveAttribute("aria-label", "Collapse sidebar");
    expect(btn).toHaveAttribute("aria-expanded", "true");
  });

  it("flips the toggle label to 'Expand sidebar' when collapsed", () => {
    renderSidebar({ onToggleCollapse: vi.fn(), isCollapsed: true });

    const btn = screen.getByTestId("sidebar-collapse-trigger");
    expect(btn).toHaveAttribute("aria-label", "Expand sidebar");
    expect(btn).toHaveAttribute("aria-expanded", "false");
  });

  it("calls onToggleCollapse when the PanelLeft button is clicked", async () => {
    const user = userEvent.setup();
    const onToggleCollapse = vi.fn();
    renderSidebar({ onToggleCollapse });

    await user.click(screen.getByTestId("sidebar-collapse-trigger"));

    expect(onToggleCollapse).toHaveBeenCalledTimes(1);
  });
});

describe("SidebarNav — icon rail (collapsed)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("renders nav icons by their aria-label (no visible text labels)", () => {
    renderSidebar({ isCollapsed: true, onToggleCollapse: vi.fn() });

    // Catalog and Instances are core items always present.
    expect(screen.getByLabelText("Catalog")).toBeInTheDocument();
    expect(screen.getByLabelText("Instances")).toBeInTheDocument();
    // Knodex wordmark hidden in rail mode.
    expect(screen.queryByText("Knodex")).not.toBeInTheDocument();
  });

  it("still renders the user-menu trigger (avatar only) at the bottom of the rail", () => {
    renderSidebar({ isCollapsed: true, onToggleCollapse: vi.fn() });

    const trigger = screen.getByTestId("user-menu-trigger");
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-label", expect.stringMatching(/Account:/));
  });

  it("keeps the PanelLeft toggle visible in the rail", () => {
    renderSidebar({ isCollapsed: true, onToggleCollapse: vi.fn() });

    expect(screen.getByTestId("sidebar-collapse-trigger")).toBeInTheDocument();
  });
});
