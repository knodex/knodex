// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import type { ReactNode } from "react";

import { LicenseSettings } from "./LicenseSettings";
import * as licenseApi from "@/api/license";
import * as authApi from "@/api/auth";
import type { LicenseStatus } from "@/types/license";

vi.mock("@/api/license");
vi.mock("@/api/auth");
vi.mock("@/hooks/useAuth", () => ({
  useIsAuthenticated: () => true,
}));

const originalEnterprise = globalThis.__ENTERPRISE__;

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <MemoryRouter>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </MemoryRouter>
    );
  }
  return render(<LicenseSettings />, { wrapper: Wrapper });
}

function mockStatus(overrides: Partial<LicenseStatus> = {}): LicenseStatus {
  return {
    licensed: true,
    enterprise: true,
    status: "valid",
    message: "License is valid",
    license: {
      licenseId: "lic_test_123",
      customer: "Acme Corp",
      edition: "enterprise",
      features: ["compliance", "audit", "sso"],
      maxUsers: 250,
      issuedAt: "2026-01-01T00:00:00Z",
      expiresAt: "2026-08-31T00:00:00Z",
    },
    ...overrides,
  };
}

describe("LicenseSettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = true;
    // Default: user is an admin (can update settings)
    vi.mocked(authApi.canI).mockResolvedValue(true);
  });

  afterEach(() => {
    (globalThis as Record<string, unknown>).__ENTERPRISE__ = originalEnterprise;
  });

  it("renders the header triad (title, Active badge, Renew license button) for a valid license", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(mockStatus());

    renderPage();

    // Title sits in the card header per the design reference, not in the sub-nav rail.
    expect(
      await screen.findByRole("heading", { name: /license status/i, level: 3 })
    ).toBeInTheDocument();
    expect(await screen.findByText("Active")).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: /renew license/i })
    ).toBeInTheDocument();
  });

  it("renders the fixed 'License is active' headline + 'Activated for {customer} · expires {date}.' subline", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(mockStatus());

    renderPage();

    expect(await screen.findByText("License is active")).toBeInTheDocument();
    // formatDate produces locale-formatted long-form; matching just the
    // structural parts keeps the test resilient across locales.
    const subline = await screen.findByText(/Activated for Acme Corp/);
    expect(subline).toHaveTextContent(/expires/);
    expect(subline.textContent ?? "").toMatch(/Acme Corp · expires .+\./);
  });

  it("renders the Grace period branch with badge, headline, and grace-end suffix", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        status: "grace_period",
        gracePeriodEnd: "2026-09-30T00:00:00Z",
      })
    );

    renderPage();

    expect(await screen.findByText("Grace period")).toBeInTheDocument();
    expect(
      await screen.findByText("License is in grace period")
    ).toBeInTheDocument();
    expect(
      await screen.findByText(/grace period ends/i)
    ).toBeInTheDocument();
  });

  it("renders the empty 'No license installed' state when no license is present", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({ status: "missing", licensed: false, license: undefined })
    );

    renderPage();

    // Empty-state copy is the centered body, NOT the banner — banner is
    // suppressed because there is no license to summarise.
    expect(
      await screen.findByText("No license installed")
    ).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: /activate license/i })
    ).toBeInTheDocument();
    expect(screen.queryByText("License is active")).not.toBeInTheDocument();
  });

  it("hides the Renew license button when the user lacks settings:update permission", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(mockStatus());
    vi.mocked(authApi.canI).mockResolvedValue(false);

    renderPage();

    // Wait for the header to settle (badge proves the page rendered)
    await screen.findByText("Active");
    // Wait for the can-i query to flip allowed → false, then assert the
    // action button is gone.
    await waitFor(() => {
      expect(
        screen.queryByRole("button", { name: /renew license|activate license/i })
      ).not.toBeInTheDocument();
    });
  });

  // ─── STORY-465: seat usage rendering ─────────────────────────────────────

  it("renders 'Active users: calculating…' and no banner when seats are absent (cold start)", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(mockStatus());
    renderPage();

    const cell = await screen.findByTestId("license-seats-active-users");
    expect(cell).toHaveTextContent("calculating…");
    expect(screen.queryByTestId("license-seats-banner")).not.toBeInTheDocument();
  });

  it("renders 'Active users: U / N' and no banner at the 'ok' threshold", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        seats: {
          used: 100,
          allowed: 250,
          windowDays: 30,
          percent: 0.4,
          threshold: "ok",
          lastUpdated: "2026-05-30T12:00:00Z",
          advisoryOnly: false,
        },
      })
    );
    renderPage();

    const cell = await screen.findByTestId("license-seats-active-users");
    expect(cell).toHaveTextContent("100 / 250");
    expect(screen.queryByTestId("license-seats-banner")).not.toBeInTheDocument();
  });

  it("renders the amber warn banner with usage copy at the 'warn' threshold", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        seats: {
          used: 210,
          allowed: 250,
          windowDays: 30,
          percent: 0.84,
          threshold: "warn",
          lastUpdated: "2026-05-30T12:00:00Z",
          advisoryOnly: false,
        },
      })
    );
    renderPage();

    const banner = await screen.findByTestId("license-seats-banner");
    expect(banner).toHaveAttribute("data-threshold", "warn");
    expect(banner).toHaveTextContent("Approaching seat limit");
    expect(banner).toHaveTextContent(/210 of 250 licensed users active in the last 30 days/);
    expect(banner).toHaveTextContent("84%");

    const cell = await screen.findByTestId("license-seats-active-users");
    expect(cell).toHaveTextContent("210 / 250");
  });

  it("renders the destructive banner with sales contact at the 'exceeded' threshold", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        seats: {
          used: 260,
          allowed: 250,
          windowDays: 30,
          percent: 1.04,
          threshold: "exceeded",
          lastUpdated: "2026-05-30T12:00:00Z",
          advisoryOnly: false,
        },
      })
    );
    renderPage();

    const banner = await screen.findByTestId("license-seats-banner");
    expect(banner).toHaveAttribute("data-threshold", "exceeded");
    expect(banner).toHaveTextContent("Seat limit exceeded");
    expect(banner).toHaveTextContent("sales@knodex.io");
  });

  it("renders 'U (Unlimited)' for an unlimited license (maxUsers === 0)", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        license: { ...mockStatus().license!, maxUsers: 0 },
        seats: {
          used: 1234,
          allowed: 0,
          windowDays: 30,
          percent: 0,
          threshold: "ok",
          lastUpdated: "2026-05-30T12:00:00Z",
          advisoryOnly: false,
        },
      })
    );
    renderPage();

    const cell = await screen.findByTestId("license-seats-active-users");
    expect(cell).toHaveTextContent("1234 (Unlimited)");
  });

  it("substitutes the cloud-tenant advisory copy on the banner when advisoryOnly=true", async () => {
    vi.mocked(licenseApi.getLicenseStatus).mockResolvedValue(
      mockStatus({
        seats: {
          used: 210,
          allowed: 250,
          windowDays: 30,
          percent: 0.84,
          threshold: "warn",
          lastUpdated: "2026-05-30T12:00:00Z",
          advisoryOnly: true,
        },
      })
    );
    renderPage();

    const banner = await screen.findByTestId("license-seats-banner");
    expect(banner).toHaveTextContent(/control plane/);
    // The literal-usage copy must NOT appear in advisory mode.
    expect(banner).not.toHaveTextContent(/Consider upgrading your license/);
  });
});
