// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { SettingsLayout } from "./SettingsLayout";
import { getSettingsNavItems, groupSettingsNavItems, resolveActiveSettingsId, type SettingsNavItem } from "./settings-nav";
import { Box } from "@/lib/icons";

// isEnterprise drives the License/Audit menu entries.
vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: vi.fn(() => false),
}));
import { isEnterprise } from "@/hooks/useCompliance";
const mockIsEnterprise = vi.mocked(isEnterprise);

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SettingsLayout>
        <div>panel content</div>
      </SettingsLayout>
    </MemoryRouter>,
  );
}

function menu() {
  return screen.getByRole("navigation", { name: /settings sections/i });
}

describe("getSettingsNavItems", () => {
  beforeEach(() => vi.clearAllMocks());

  it("always includes General and SSO Providers", () => {
    mockIsEnterprise.mockReturnValue(false);
    const ids = getSettingsNavItems().map((i) => i.id);
    expect(ids).toContain("general");
    expect(ids).toContain("sso");
  });

  it("omits enterprise items in a non-enterprise build", () => {
    mockIsEnterprise.mockReturnValue(false);
    const ids = getSettingsNavItems().map((i) => i.id);
    expect(ids).not.toContain("license");
    expect(ids).not.toContain("audit");
  });

  it("includes License and Audit in an enterprise build", () => {
    mockIsEnterprise.mockReturnValue(true);
    const ids = getSettingsNavItems().map((i) => i.id);
    expect(ids).toContain("license");
    expect(ids).toContain("audit");
  });

  it("emits no commerce/billing items", () => {
    mockIsEnterprise.mockReturnValue(true);
    const ids = getSettingsNavItems().map((i) => i.id);
    expect(ids).not.toContain("plan");
    expect(ids).not.toContain("billing");
    expect(ids).not.toContain("marketplace");
  });

  it("surfaces the federated Teams item and emits no Members item", () => {
    mockIsEnterprise.mockReturnValue(true);
    const items = getSettingsNavItems();
    const ids = items.map((i) => i.id);
    const labels = items.map((i) => i.label);
    expect(ids).toContain("teams");
    expect(labels).toContain("Teams");
    expect(labels).not.toContain("Members");
  });

  it("emits no two nav items sharing a label (OSS/EE build)", () => {
    mockIsEnterprise.mockReturnValue(true);
    const labels = getSettingsNavItems().map((i) => i.label);
    expect(new Set(labels).size).toBe(labels.length);
  });

  it("marks only General as an exact match", () => {
    const general = getSettingsNavItems().find((i) => i.id === "general");
    expect(general?.exact).toBe(true);
    expect(general?.to).toBe("/settings");
  });
});

describe("groupSettingsNavItems", () => {
  beforeEach(() => vi.clearAllMocks());

  it("leads with the ungrouped General item in a headerless section", () => {
    mockIsEnterprise.mockReturnValue(false);
    const groups = groupSettingsNavItems(getSettingsNavItems());
    expect(groups[0].label).toBeNull();
    expect(groups[0].items.map((i) => i.id)).toEqual(["general"]);
  });

  it("buckets identity/RBAC items under the Access group in intra-group order", () => {
    mockIsEnterprise.mockReturnValue(false);
    const groups = groupSettingsNavItems(getSettingsNavItems());
    const access = groups.find((g) => g.id === "access");
    expect(access?.label).toBe("Access");
    expect(access?.items.map((i) => i.id)).toEqual(["sso", "role-templates", "users", "teams"]);
  });

  it("drops empty groups (no Billing/Platform headers in the non-enterprise OSS build)", () => {
    mockIsEnterprise.mockReturnValue(false);
    const labels = groupSettingsNavItems(getSettingsNavItems()).map((g) => g.label);
    expect(labels).toEqual([null, "Access"]);
  });

  it("adds the Platform group (License/Audit) in an enterprise build", () => {
    mockIsEnterprise.mockReturnValue(true);
    const groups = groupSettingsNavItems(getSettingsNavItems());
    const platform = groups.find((g) => g.id === "platform");
    expect(platform?.label).toBe("Platform");
    expect(platform?.items.map((i) => i.id)).toEqual(["license", "audit"]);
    // Still no Billing group without the cloud build.
    expect(groups.map((g) => g.id)).not.toContain("billing");
  });

  it("renders the Access group header in the settings nav", () => {
    mockIsEnterprise.mockReturnValue(true);
    renderAt("/settings");
    expect(within(menu()).getByText("Access")).toBeInTheDocument();
    expect(within(menu()).getByText("Platform")).toBeInTheDocument();
  });
});

describe("resolveActiveSettingsId", () => {
  // Synthetic menu with alias/prefix cases so the resolver's logic can be
  // exercised directly, independent of the live nav model.
  const items: SettingsNavItem[] = [
    { id: "general", label: "General", icon: Box, to: "/settings", exact: true },
    { id: "sso", label: "SSO", icon: Box, to: "/settings/sso" },
    { id: "plan", label: "Plan", icon: Box, to: "/settings/plan", aliases: ["/settings/billing/plan"] },
    { id: "billing", label: "Billing", icon: Box, to: "/settings/billing" },
  ];

  it("resolves General only on the exact /settings path", () => {
    expect(resolveActiveSettingsId("/settings", items)).toBe("general");
  });

  it("resolves an exact sub-path to its item", () => {
    expect(resolveActiveSettingsId("/settings/billing", items)).toBe("billing");
  });

  it("keeps the parent active on a deeper sub-route (e.g. SSO form)", () => {
    expect(resolveActiveSettingsId("/settings/sso/new", items)).toBe("sso");
  });

  it("resolves the /settings/billing/plan alias to Plan, not Billing", () => {
    expect(resolveActiveSettingsId("/settings/billing/plan", items)).toBe("plan");
  });

  it("does not let Billing claim the deeper billing/plan alias path", () => {
    // Sanity: the alias path must not resolve to billing via prefix matching.
    expect(resolveActiveSettingsId("/settings/billing/plan", items)).not.toBe("billing");
  });

  it("returns null when nothing matches", () => {
    expect(resolveActiveSettingsId("/instances", items)).toBeNull();
  });
});

describe("SettingsLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockIsEnterprise.mockReturnValue(true);
  });

  it("renders the menu and content panel (Settings title owned by topbar after Story 48.12)", () => {
    renderAt("/settings");
    // Page identity ("Settings") now lives in the topbar breadcrumb leaf, tested
    // in `web/src/components/layout/Breadcrumbs.test.tsx`.
    expect(menu()).toBeInTheDocument();
    expect(screen.getByText("panel content")).toBeInTheDocument();
  });

  it("highlights General on /settings (exact) without highlighting SSO", () => {
    renderAt("/settings");
    const nav = menu();
    expect(within(nav).getByRole("link", { name: /general/i })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: /sso providers/i })).not.toHaveAttribute("aria-current");
  });

  it("highlights the SSO item on /settings/sso and not General", () => {
    renderAt("/settings/sso");
    const nav = menu();
    expect(within(nav).getByRole("link", { name: /sso providers/i })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: /general/i })).not.toHaveAttribute("aria-current");
  });

  it("renders the side menu at 180px and sticky on md+ (Story 48.10 AC #2)", () => {
    renderAt("/settings");
    const nav = menu();
    // 180px width (down from md:w-56) so the rail is slimmer per the prototype.
    expect(nav).toHaveClass("md:w-[180px]");
    // Sticky on md+ so the rail follows on scroll; self-start prevents the flex
    // child stretching full height (which would defeat position: sticky).
    expect(nav).toHaveClass("md:sticky");
    expect(nav).toHaveClass("md:top-[72px]");
    expect(nav).toHaveClass("md:self-start");
    // Small-screen horizontal-scroll behavior preserved.
    expect(nav).toHaveClass("overflow-x-auto");
  });
});
