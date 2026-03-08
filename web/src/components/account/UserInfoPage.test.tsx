// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { AccountInfoResponse } from "@/api/auth";

// Mock the API module
let mockAccountInfo: AccountInfoResponse | null = null;
let mockAccountInfoError: Error | null = null;

vi.mock("@/api/auth", () => ({
  getAccountInfo: vi.fn(() => {
    if (mockAccountInfoError) return Promise.reject(mockAccountInfoError);
    if (mockAccountInfo) return Promise.resolve(mockAccountInfo);
    // Default: return null (no API data, falls back to store)
    return Promise.reject(new Error("not configured"));
  }),
}));

/** Build a minimal AccountInfoResponse for test mocks */
function buildAccountInfo(overrides: Partial<AccountInfoResponse> = {}): AccountInfoResponse {
  return {
    userID: "test-user",
    email: "test@example.com",
    displayName: "Test User",
    groups: [],
    casbinRoles: ["role:serveradmin"],
    projects: [],
    roles: {},
    issuer: "https://auth.example.com",
    tokenExpiresAt: Math.floor(Date.now() / 1000) + 3600,
    tokenIssuedAt: Math.floor(Date.now() / 1000) - 600,
    ...overrides,
  };
}

// Default mock state for an OIDC user
const oidcUserState = {
  user: { id: "oidc-user-123", email: "user@example.com", name: "Test User" },
  groups: ["engineering", "platform-team"],
  casbinRoles: ["role:serveradmin"],
  projects: ["proj-alpha", "proj-beta"],
  roles: { "proj-alpha": "developer", "proj-beta": "viewer" } as Record<string, string>,
  issuer: "https://auth.example.com",
  tokenExp: Math.floor(Date.now() / 1000) + 3600, // 60 min from now
  tokenIat: Math.floor(Date.now() / 1000) - 600,  // 10 min ago
};

// Local admin mock state
const localAdminState = {
  user: { id: "local-admin-001", email: "admin@local", name: "Admin User" },
  groups: [] as string[],
  casbinRoles: ["role:serveradmin"],
  projects: ["default"],
  roles: {} as Record<string, string>,
  issuer: null as string | null,
  tokenExp: Math.floor(Date.now() / 1000) + 3600,
  tokenIat: Math.floor(Date.now() / 1000) - 600,
};

let mockState = { ...oidcUserState };

vi.mock("@/stores/userStore", () => ({
  useUserStore: vi.fn((selector: (state: typeof mockState) => unknown) =>
    selector(mockState)
  ),
}));

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
}

describe("UserInfoPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockState = { ...oidcUserState };
    mockAccountInfo = null;
    mockAccountInfoError = null;
  });

  // Helper to import fresh each time (after mock is set) with QueryClientProvider
  async function renderPage() {
    const { UserInfoPage } = await import("./UserInfoPage");
    const queryClient = createQueryClient();
    return render(
      <QueryClientProvider client={queryClient}>
        <UserInfoPage />
      </QueryClientProvider>
    );
  }

  describe("OIDC user", () => {
    it("renders identity card with user info", async () => {
      await renderPage();

      expect(screen.getByText("Your identity, access level, and session details")).toBeInTheDocument();
      expect(screen.getByText("Identity")).toBeInTheDocument();
      expect(screen.getByText("Test User")).toBeInTheDocument();
      expect(screen.getByText("user@example.com")).toBeInTheDocument();
    });

    it("renders authentication card with OIDC issuer", async () => {
      await renderPage();

      expect(screen.getByText("Authentication")).toBeInTheDocument();
      expect(screen.getByText("https://auth.example.com")).toBeInTheDocument();
    });

    it("renders groups as badges from API response", async () => {
      mockAccountInfo = buildAccountInfo({ groups: ["engineering", "platform-team"] });
      await renderPage();

      expect(screen.getByText("Groups")).toBeInTheDocument();
      expect(await screen.findByText("engineering")).toBeInTheDocument();
      expect(screen.getByText("platform-team")).toBeInTheDocument();
    });

    it("falls back to store groups when API fails", async () => {
      // mockAccountInfo is null → API rejects → component falls back to store groups
      await renderPage();

      await waitFor(() => {
        expect(screen.getByText("engineering")).toBeInTheDocument();
        expect(screen.getByText("platform-team")).toBeInTheDocument();
      }, { timeout: 3000 });
    });

    it("renders global roles", async () => {
      await renderPage();

      expect(screen.getByText("Roles & Access")).toBeInTheDocument();
      expect(screen.getByText("role:serveradmin")).toBeInTheDocument();
    });

    it("renders project-scoped roles with context", async () => {
      await renderPage();

      expect(screen.getByText("developer on proj-alpha")).toBeInTheDocument();
      expect(screen.getByText("viewer on proj-beta")).toBeInTheDocument();
    });

  });

  describe("local admin user", () => {
    beforeEach(() => {
      mockState = { ...localAdminState };
    });

    it("renders identity card for local admin", async () => {
      await renderPage();

      expect(screen.getByText("Admin User")).toBeInTheDocument();
      expect(screen.getByText("admin@local")).toBeInTheDocument();
    });

    it("shows 'Local' as issuer for local admin", async () => {
      await renderPage();

      expect(screen.getByText("Local")).toBeInTheDocument();
    });

    it("shows no groups message for local admin", async () => {
      mockAccountInfo = buildAccountInfo({ groups: [] });
      await renderPage();

      expect(await screen.findByText("Local admin users have no OIDC groups")).toBeInTheDocument();
    });

    it("shows no project roles for local admin", async () => {
      await renderPage();

      // Should not render a Project Roles section since roles is empty
      expect(screen.queryByText("Project Roles")).not.toBeInTheDocument();
    });
  });

  describe("unauthenticated state", () => {
    it("shows not authenticated message when no user", async () => {
      mockState = {
        ...oidcUserState,
        user: null as unknown as typeof oidcUserState.user,
      };

      await renderPage();

      expect(screen.getByText("Not authenticated")).toBeInTheDocument();
    });
  });

  describe("edge cases", () => {
    it("handles OIDC user with no groups", async () => {
      mockState = {
        ...oidcUserState,
        groups: [],
      };
      mockAccountInfo = buildAccountInfo({ groups: [] });

      await renderPage();

      expect(await screen.findByText("No groups assigned")).toBeInTheDocument();
    });

    it("handles user with no casbin roles", async () => {
      mockState = {
        ...oidcUserState,
        casbinRoles: [],
      };

      await renderPage();

      expect(screen.getByText("No global roles")).toBeInTheDocument();
    });

    it("falls back to email prefix for display name when name is missing", async () => {
      mockState = {
        ...oidcUserState,
        user: { id: "user@example.com", email: "user@example.com" } as typeof oidcUserState.user,
      };

      await renderPage();

      // Display Name should show "user" (email prefix) since name is undefined
      expect(screen.getByText("user")).toBeInTheDocument();
    });
  });
});
