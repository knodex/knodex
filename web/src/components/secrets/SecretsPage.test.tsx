// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SecretsPage } from "./SecretsPage";
import * as useSecretsModule from "@/hooks/useSecrets";
import * as useProjectsModule from "@/hooks/useProjects";
import * as useCanIModule from "@/hooks/useCanI";

vi.mock("@/hooks/useSecrets", () => ({
  useSecretList: vi.fn(),
  useCreateSecret: vi.fn(),
  useDeleteSecret: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}));

vi.mock("@/hooks/useProjects", () => ({
  useProjects: vi.fn(),
}));

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(),
}));

vi.mock("@/hooks/useAuth", () => ({
  useCurrentProject: vi.fn(() => "alpha"),
}));

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

function renderPage(initialRoute = "/secrets") {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter initialEntries={[initialRoute]}>
          <SecretsPage />
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

const mockProjects = {
  items: [{ name: "alpha", description: "Alpha project", destinations: [], roles: [] }],
  totalCount: 1,
};

const mockSecrets = {
  items: [
    {
      name: "db-credentials",
      namespace: "alpha-ns",
      keys: ["username", "password"],
      createdAt: "2026-03-15T10:00:00Z",
    },
    {
      name: "api-key",
      namespace: "alpha-ns",
      keys: ["key"],
      createdAt: "2026-03-16T12:00:00Z",
    },
  ],
  pageCount: 2,
};

describe("SecretsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders table with secret data", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });
    expect(screen.getByText("api-key")).toBeInTheDocument();
    expect(screen.getByText("username, password")).toBeInTheDocument();
    expect(screen.getByText("key")).toBeInTheDocument();
  });

  it("renders ListFooter with counts computed over the visible rows", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    // Fixture: 2 secrets, keys 2+1=3. (Distinct-namespace count was dropped;
    // each secret lives in exactly one namespace so an aggregate is misleading.)
    const footer = await screen.findByTestId("secrets-list-footer");
    expect(footer).toHaveTextContent("2 secrets");
    expect(footer).toHaveTextContent("3 keys");
    expect(footer).not.toHaveTextContent(/namespaces?/);
  });

  it("footer counts track the search-filtered rows, not the full list", async () => {
    const user = userEvent.setup();

    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    const footer = await screen.findByTestId("secrets-list-footer");
    expect(footer).toHaveTextContent("2 secrets");

    // Narrow the search to a single secret (db-credentials → 2 keys, alpha-ns).
    // The footer must follow the visible (filtered) `sorted` array; if it counted
    // the unfiltered `data.items` it would still read "2 secrets" and fail here.
    await user.type(screen.getByLabelText("Search secrets"), "db-credentials");

    await waitFor(() => {
      expect(screen.getByTestId("secrets-list-footer")).toHaveTextContent("1 secrets");
    });
    expect(screen.getByTestId("secrets-list-footer")).toHaveTextContent("2 keys");
    expect(screen.getByTestId("secrets-list-footer")).not.toHaveTextContent(/namespaces?/);
  });

  it("shows loading skeletons", () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    expect(document.querySelectorAll(".animate-token-shimmer").length).toBeGreaterThan(0);
  });

  it("shows empty state when no secrets", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: { items: [], pageCount: 0 },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("No secrets yet")).toBeInTheDocument();
    });
    // Footer must not render in the empty-state branch.
    expect(screen.queryByTestId("secrets-list-footer")).not.toBeInTheDocument();
  });

  it("shows access denied when user lacks permission", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: false,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("Access Denied")).toBeInTheDocument();
    });
  });

  it("shows error alert with retry when list fails", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    const mockRefetch = vi.fn();
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("Network error"),
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("Failed to load secrets")).toBeInTheDocument();
    });
  });

  it("hides Create button when user lacks create permission", async () => {
    // First call: secrets/get = true, Second call: secrets/create = false
    vi.mocked(useCanIModule.useCanI).mockImplementation((resource, action) => {
      if (action === "get") return { allowed: true, isLoading: false, isError: false };
      return { allowed: false, isLoading: false, isError: false };
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });
    expect(screen.queryByText("Create Secret")).not.toBeInTheDocument();
  });

  it("hides inline delete button per row when user lacks delete permission", async () => {
    vi.mocked(useCanIModule.useCanI).mockImplementation((_resource, action) => {
      if (action === "delete") return { allowed: false, isLoading: false, isError: false };
      return { allowed: true, isLoading: false, isError: false };
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });
    expect(screen.queryByLabelText("Delete db-credentials")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Delete api-key")).not.toBeInTheDocument();
  });

  it("shows inline delete button per row when user has delete permission", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });
    expect(screen.getByLabelText("Delete db-credentials")).toBeInTheDocument();
    expect(screen.getByLabelText("Delete api-key")).toBeInTheDocument();
  });

  it("calls useSecretList with no namespace filter (server-filtered to accessible namespaces)", async () => {
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: {
        items: [
          { name: "alpha", description: "", destinations: [], roles: [] },
          { name: "beta", description: "", destinations: [], roles: [] },
        ],
        totalCount: 2,
      },
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });
    // The page makes a single namespace-agnostic list call — secrets are
    // namespace-keyed and the server filters to accessible namespaces.
    expect(useSecretsModule.useSecretList).toHaveBeenCalledWith();
  });

  it("navigates to detail view with correct URL on row click", async () => {
    const user = userEvent.setup();

    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(useProjectsModule.useProjects).mockReturnValue({
      data: mockProjects,
      isLoading: false,
    } as ReturnType<typeof useProjectsModule.useProjects>);
    vi.mocked(useSecretsModule.useSecretList).mockReturnValue({
      data: mockSecrets,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useSecretsModule.useSecretList>);
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);

    renderPage();

    await waitFor(() => {
      expect(screen.getByText("db-credentials")).toBeInTheDocument();
    });

    // Click on the first secret row
    await user.click(screen.getByText("db-credentials"));

    expect(mockNavigate).toHaveBeenCalledWith(
      "/secrets/alpha-ns/db-credentials"
    );
  });
});
