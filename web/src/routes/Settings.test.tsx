// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { Settings } from "./Settings";

const mockUseSettings = vi.fn();
const mockUseUserStore = vi.fn();
const mockIsEnterprise = vi.fn();

vi.mock("@/hooks/useSettings", () => ({
  useSettings: () => mockUseSettings(),
}));

vi.mock("@/hooks/useCompliance", () => ({
  isEnterprise: () => mockIsEnterprise(),
}));

vi.mock("@/stores/userStore", () => ({
  useUserStore: (selector: (s: { issuer: string | null }) => unknown) =>
    selector(mockUseUserStore() as { issuer: string | null }),
}));

beforeEach(() => {
  vi.clearAllMocks();
  mockUseSettings.mockReturnValue({
    data: { organization: "acme" },
    isLoading: false,
  });
  mockUseUserStore.mockReturnValue({ issuer: "Local" });
  mockIsEnterprise.mockReturnValue(false);
});

describe("Settings (General) — prototype-aligned layout", () => {
  it("renders an Organisation card with Name row", () => {
    render(<Settings />);
    expect(screen.getByText("Organisation")).toBeInTheDocument();
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("acme")).toBeInTheDocument();
  });

  it("renders a Platform card with Version / Edition rows", () => {
    render(<Settings />);
    expect(screen.getByText("Platform")).toBeInTheDocument();
    expect(screen.getByText("Version")).toBeInTheDocument();
    // version uses the __APP_VERSION__ build literal (defaults to "dev" when unset)
    expect(screen.getByText(/^v/)).toBeInTheDocument();
    expect(screen.getByText("Edition")).toBeInTheDocument();
    expect(screen.getByText("OSS")).toBeInTheDocument();
  });

  it("shows Enterprise when the build is enterprise", () => {
    mockIsEnterprise.mockReturnValue(true);
    render(<Settings />);
    expect(screen.getByText("Enterprise")).toBeInTheDocument();
    expect(screen.queryByText("OSS")).not.toBeInTheDocument();
  });

  it("falls back to em-dash when settings has no organization yet", () => {
    mockUseSettings.mockReturnValue({ data: undefined, isLoading: false });
    render(<Settings />);
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("does not fabricate prototype-only fields (Region, Plan, Created, Default project)", () => {
    // These fields appear in the design prototype but the backend doesn't carry
    // them. Regression guard against accidentally re-introducing mock data.
    render(<Settings />);
    expect(screen.queryByText(/Region/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Plan/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Created/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Default project/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Control plane/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/Danger zone/i)).not.toBeInTheDocument();
  });
});
