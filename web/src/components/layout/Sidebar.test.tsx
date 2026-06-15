// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { SidebarNav } from "./Sidebar";

// Mock hooks consumed by SidebarNav.
vi.mock("@/hooks/useRGDs", () => ({
  useRGDList: () => ({ data: undefined }),
}));

interface MockComplianceSummary {
  totalTemplates: number;
  totalConstraints: number;
  totalViolations: number;
  byEnforcement: Record<string, number>;
}
const mockViolationCount = vi.fn(() => ({ data: 0 as number | undefined }));
const mockComplianceSummary = vi.fn(() => ({
  data: undefined as MockComplianceSummary | undefined,
}));
const mockIsEnterprise = vi.fn(() => false);
vi.mock("@/hooks/useCompliance", () => ({
  useViolationCount: () => mockViolationCount(),
  useComplianceSummary: () => mockComplianceSummary(),
  isEnterprise: () => mockIsEnterprise(),
}));

vi.mock("@/hooks/useCategories", () => ({
  useCategoriesEnabled: () => ({ enabled: false, isLoading: false, categories: [] }),
}));

// Map of resource -> allowed bool, defaulting to false. Tests override via
// mockUseCanI.mockImplementation when they need specific permissions.
const mockUseCanI = vi.fn((_resource: string, _action: string, _scope: string) => ({
  allowed: false,
}));
vi.mock("@/hooks/useCanI", () => ({
  useCanI: (resource: string, action: string, scope: string) =>
    mockUseCanI(resource, action, scope),
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
  route?: string;
} = {}) {
  return render(
    <MemoryRouter initialEntries={[opts.route ?? "/"]}>
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

describe("SidebarNav — Settings is not a manageItems entry", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsEnterprise.mockReturnValue(false);
    mockViolationCount.mockReturnValue({ data: 0 });
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  // Settings was removed from the sidebar Manage section; it is reached only
  // through the user-menu dropdown (covered in "SidebarNav — UserMenu").
  it("never renders a Settings link in the Manage section, even when settings/* get is allowed", () => {
    mockUseCanI.mockImplementation((resource: string) => ({
      allowed: resource === "settings",
    }));

    renderSidebar();

    const manageSection = screen.getByRole("group", { name: "Manage" });
    const labels = Array.from(
      manageSection.querySelectorAll<HTMLAnchorElement>("a[href]")
    ).map((a) => a.textContent?.trim());
    expect(labels).not.toContain("Settings");
  });
});

describe("SidebarNav — Compliance amber badge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsEnterprise.mockReturnValue(true);
    mockUseCanI.mockImplementation(() => ({ allowed: false }));
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("fills the Compliance badge with the amber warning tokens when violationCount > 0", () => {
    mockViolationCount.mockReturnValue({ data: 5 });

    renderSidebar();

    const complianceLink = screen.getByRole("link", { name: "Compliance" });
    // Badge sits inside the Compliance link; find by text content "5".
    const badge = within(complianceLink).getByText("5");
    expect(badge.getAttribute("style")).toMatch(/hsl\(var\(--status-warning-hsl\)/);
    expect(badge.getAttribute("style")).toMatch(/var\(--status-warning\)/);
  });

  it("renders no badge on Compliance when violationCount === 0 (existing behavior preserved)", () => {
    mockViolationCount.mockReturnValue({ data: 0 });

    renderSidebar();

    const complianceLink = screen.getByRole("link", { name: "Compliance" });
    // The aria-label badge marker only appears when badge > 0.
    expect(within(complianceLink).queryByLabelText(/items$/)).not.toBeInTheDocument();
  });
});

describe("SidebarNav — Compliance sub-nav count chips", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsEnterprise.mockReturnValue(true);
    mockUseCanI.mockImplementation(() => ({ allowed: false }));
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("shows neutral count chips on Templates/Constraints and the amber count on Violations", () => {
    mockViolationCount.mockReturnValue({ data: 3 });
    mockComplianceSummary.mockReturnValue({
      data: { totalTemplates: 12, totalConstraints: 7, totalViolations: 3, byEnforcement: {} },
    });

    renderSidebar({ route: "/compliance" });

    // The sub-nav Link has no explicit aria-label, so the badge text folds into
    // the accessible name (e.g. "Templates 12") — the same behavior the existing
    // Violations sub-nav badge has today. Match the label prefix, assert the chip.
    const templates = screen.getByRole("link", { name: /^Templates/ });
    expect(within(templates).getByText("12")).toBeInTheDocument();

    const constraints = screen.getByRole("link", { name: /^Constraints/ });
    expect(within(constraints).getByText("7")).toBeInTheDocument();

    const violations = screen.getByRole("link", { name: /^Violations/ });
    expect(within(violations).getByText("3")).toBeInTheDocument();
  });

  it("renders no count chip on Templates/Constraints when their counts are 0 (> 0 guard)", () => {
    mockViolationCount.mockReturnValue({ data: 0 });
    mockComplianceSummary.mockReturnValue({
      data: { totalTemplates: 0, totalConstraints: 0, totalViolations: 0, byEnforcement: {} },
    });

    renderSidebar({ route: "/compliance" });

    // With 0 counts the chip is suppressed by the `badge > 0` guard, so the
    // accessible name is just the label and no digit chip renders.
    const templates = screen.getByRole("link", { name: /^Templates/ });
    expect(within(templates).queryByText("0")).not.toBeInTheDocument();

    const constraints = screen.getByRole("link", { name: /^Constraints/ });
    expect(within(constraints).queryByText("0")).not.toBeInTheDocument();
  });

  it("never renders a count badge on the Overview sub-nav link", () => {
    mockViolationCount.mockReturnValue({ data: 3 });
    mockComplianceSummary.mockReturnValue({
      data: { totalTemplates: 12, totalConstraints: 7, totalViolations: 3, byEnforcement: {} },
    });

    renderSidebar({ route: "/compliance" });

    const overview = screen.getByRole("link", { name: /^Overview/ });
    // Overview carries no numeric chip — no digit-only badge span inside it.
    expect(within(overview).queryByText(/^\d+$/)).not.toBeInTheDocument();
  });
});

describe("SidebarNav — cloud pages live under Settings, not the sidebar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("renders no 'Cloud' section group", () => {
    renderSidebar();
    expect(screen.queryByRole("group", { name: "Cloud" })).not.toBeInTheDocument();
  });

  it("renders no Plan/Billing/Marketplace/Team nav links", () => {
    renderSidebar();
    for (const label of ["Plan", "Billing", "Marketplace", "Team"]) {
      expect(screen.queryByRole("link", { name: label })).not.toBeInTheDocument();
    }
  });
});

describe("SidebarNav — Agents sub-sidebar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseAuth.mockReturnValue({
      user: mockUser,
      isAuthenticated: true,
      login: vi.fn(),
      logout: mockLogout,
    });
  });

  it("renders the agents secondary sidebar with Overview/Agents/Models and a Back link", () => {
    renderSidebar({ route: "/agents" });

    const nav = screen.getByRole("navigation", { name: /agents navigation/i });
    expect(within(nav).getByRole("link", { name: /^back$/i })).toHaveAttribute("href", "/instances");
    expect(within(nav).getByRole("link", { name: /overview/i })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: /^agents$/i })).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: /models/i })).toBeInTheDocument();
  });

  it("renders no count badges in the agents sub-sidebar", () => {
    renderSidebar({ route: "/agents/list" });

    const nav = screen.getByRole("navigation", { name: /agents navigation/i });
    // No numeric chips anywhere in the agents nav.
    expect(within(nav).queryByText(/^\d+$/)).not.toBeInTheDocument();
  });

  it("highlights Overview on exact /agents without highlighting Agents", () => {
    renderSidebar({ route: "/agents" });
    const nav = screen.getByRole("navigation", { name: /agents navigation/i });
    expect(within(nav).getByRole("link", { name: /overview/i })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: /^agents$/i })).not.toHaveAttribute("aria-current");
  });

  it("highlights the Agents tab on /agents/list and not Overview", () => {
    renderSidebar({ route: "/agents/list" });
    const nav = screen.getByRole("navigation", { name: /agents navigation/i });
    expect(within(nav).getByRole("link", { name: /^agents$/i })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: /overview/i })).not.toHaveAttribute("aria-current");
  });

  it("keeps Agents highlighted on a deep chat route", () => {
    renderSidebar({ route: "/agents/list/kagent/rgd-builder/chat/abc" });
    const nav = screen.getByRole("navigation", { name: /agents navigation/i });
    expect(within(nav).getByRole("link", { name: /^agents$/i })).toHaveAttribute("aria-current", "page");
  });
});
