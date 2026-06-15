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
};

// Member with no project bindings (story 17.1) — the canonical "bare member"
// who must land on the self-view, not a 403 wall.
const bareMemberState = {
  user: { id: "oidc-member-001", email: "newcomer@example.com", name: "New Comer" },
  groups: [] as string[],
  casbinRoles: [] as string[],
  projects: [] as string[],
  roles: {} as Record<string, string>,
  issuer: "https://auth.example.com",
};

// Local admin mock state
const localAdminState = {
  user: { id: "local-admin-001", email: "admin@local", name: "Admin User" },
  groups: [] as string[],
  casbinRoles: ["role:serveradmin"],
  projects: ["default"],
  roles: {} as Record<string, string>,
  issuer: null as string | null,
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

  describe("My Access (story 17.1)", () => {
    it("renders the My Access header and self-view wrapper", async () => {
      await renderPage();

      expect(screen.getByTestId("my-access-view")).toBeInTheDocument();
      expect(screen.getByRole("heading", { name: "My Access" })).toBeInTheDocument();
    });

    it("shows the serveradmin application-role badge for a server admin", async () => {
      mockAccountInfo = buildAccountInfo({ applicationRole: "serveradmin" });
      await renderPage();

      const badge = await screen.findByTestId("application-role");
      expect(badge).toHaveTextContent("serveradmin");
    });

    it("shows the member application-role badge for a bare member", async () => {
      mockState = { ...bareMemberState };
      mockAccountInfo = buildAccountInfo({
        userID: "oidc-member-001",
        email: "newcomer@example.com",
        casbinRoles: [],
        projects: [],
        roles: {},
        applicationRole: "member",
      });
      await renderPage();

      const badge = await screen.findByTestId("application-role");
      expect(badge).toHaveTextContent("member");
    });

    it("renders the bound-projects section with project roles from account info", async () => {
      mockState = { ...bareMemberState };
      mockAccountInfo = buildAccountInfo({
        casbinRoles: [],
        projects: ["proj-gamma"],
        roles: { "proj-gamma": "developer" },
        applicationRole: "member",
      });
      await renderPage();

      expect(await screen.findByTestId("bound-projects")).toBeInTheDocument();
      expect(screen.getByText("developer on proj-gamma")).toBeInTheDocument();
      // Plain-language effective-access summary, not raw Casbin tuples
      expect(screen.getByTestId("effective-access-summary")).toBeInTheDocument();
      expect(screen.getByText(/You can act as/)).toBeInTheDocument();
      expect(screen.queryByTestId("my-access-empty")).not.toBeInTheDocument();
    });

    it("shows the honest empty-state for a member with no bindings", async () => {
      mockState = { ...bareMemberState };
      mockAccountInfo = buildAccountInfo({
        casbinRoles: [],
        projects: [],
        roles: {},
        applicationRole: "member",
      });
      await renderPage();

      const empty = await screen.findByTestId("my-access-empty");
      expect(empty).toBeInTheDocument();
      expect(
        screen.getByText("You're signed in but not yet a member of any project")
      ).toBeInTheDocument();
      // A bare member must NOT see a Project Roles section
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
