// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SecretDetailView } from "./SecretDetailView";
import * as useCanIModule from "@/hooks/useCanI";
import * as secretsApi from "@/api/secrets";
import type { SecretDetail } from "@/types/secret";

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(),
}));

vi.mock("@/hooks/useAuth", () => ({
  useCurrentProject: vi.fn(() => "alpha"),
}));

vi.mock("@/hooks/useSecrets", () => ({
  useUpdateSecret: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteSecret: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}));

vi.mock("@/api/secrets", () => ({
  getSecret: vi.fn(),
}));

const mockSecretDetail: SecretDetail = {
  name: "db-credentials",
  namespace: "alpha-ns",
  data: {
    username: "admin",
    password: "supersecret123",
  },
  createdAt: "2026-03-15T10:00:00Z",
  labels: { app: "myapp" },
};

const createTestQueryClient = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderComponent(props?: Partial<{ name: string; namespace: string }>) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter>
          <SecretDetailView
            name={props?.name ?? "db-credentials"}
            namespace={props?.namespace ?? "alpha-ns"}
          />
        </MemoryRouter>
      </TooltipProvider>
    </QueryClientProvider>
  );
}

describe("SecretDetailView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCanIModule.useCanI).mockReturnValue({
      allowed: true,
      isLoading: false,
      isError: false,
    });
    vi.mocked(secretsApi.getSecret).mockResolvedValue(mockSecretDetail);
  });

  it("renders metadata from props without triggering an API call on mount", () => {
    renderComponent();

    expect(screen.getByText("db-credentials")).toBeInTheDocument();
    expect(screen.getByText("alpha-ns")).toBeInTheDocument();
    // No API call should have been made on mount
    expect(secretsApi.getSecret).not.toHaveBeenCalled();
  });

  it("shows 'Load values' button and informational message before loading", () => {
    renderComponent();

    expect(screen.getByRole("button", { name: /load values/i })).toBeInTheDocument();
    expect(screen.getByText(/click "load values"/i)).toBeInTheDocument();
    // No key-value rows visible yet
    expect(screen.queryByText("username")).not.toBeInTheDocument();
    expect(screen.queryByText("••••••••")).not.toBeInTheDocument();
  });

  it("fetches and displays secret data when 'Load values' is clicked", async () => {
    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(secretsApi.getSecret).toHaveBeenCalledWith("db-credentials", "alpha", "alpha-ns");
    });

    await waitFor(() => {
      expect(screen.getByText("username")).toBeInTheDocument();
      expect(screen.getByText("password")).toBeInTheDocument();
    });

    // Values should be masked by default
    expect(screen.getAllByText("••••••••")).toHaveLength(2);
    expect(screen.queryByText("admin")).not.toBeInTheDocument();
    expect(screen.queryByText("supersecret123")).not.toBeInTheDocument();

    // Key count visible in header
    expect(screen.getByText("Data (2 keys)")).toBeInTheDocument();
  });

  it("shows/hides values when toggle is clicked after loading", async () => {
    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText("username")).toBeInTheDocument();
    });

    // Click "Show username" toggle
    const showButtons = screen.getAllByLabelText(/^Show /);
    expect(showButtons.length).toBe(2);

    await user.click(showButtons[0]);

    // Username value should be visible
    expect(screen.getByText("admin")).toBeInTheDocument();
    // Password should still be masked
    expect(screen.queryByText("supersecret123")).not.toBeInTheDocument();
  });

  it("shows copy button only for revealed values", async () => {
    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText("username")).toBeInTheDocument();
    });

    // No copy button before reveal
    expect(screen.queryByLabelText("Copy username")).not.toBeInTheDocument();

    // Reveal first value
    await user.click(screen.getAllByLabelText(/^Show /)[0]);

    // Copy button should appear for revealed value only
    expect(screen.getByLabelText("Copy username")).toBeInTheDocument();
    expect(screen.queryByLabelText("Copy password")).not.toBeInTheDocument();
  });

  it("disables Edit button when values are not loaded", () => {
    renderComponent();

    const editButton = screen.getByRole("button", { name: /edit/i });
    expect(editButton).toBeDisabled();
  });

  it("enables Edit button after values are loaded", async () => {
    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText("username")).toBeInTheDocument();
    });

    const editButton = screen.getByRole("button", { name: /edit/i });
    expect(editButton).not.toBeDisabled();
  });

  it("hides Edit button without update permission", () => {
    vi.mocked(useCanIModule.useCanI).mockImplementation((_resource, action) => {
      if (action === "update") return { allowed: false, isLoading: false, isError: false };
      return { allowed: true, isLoading: false, isError: false };
    });

    renderComponent();

    expect(screen.queryByRole("button", { name: /edit/i })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /delete/i })).toBeInTheDocument();
  });

  it("hides Delete button without delete permission", () => {
    vi.mocked(useCanIModule.useCanI).mockImplementation((_resource, action) => {
      if (action === "delete") return { allowed: false, isLoading: false, isError: false };
      return { allowed: true, isLoading: false, isError: false };
    });

    renderComponent();

    expect(screen.getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
  });

  it("shows 'Retry' in header button after load error", async () => {
    vi.mocked(secretsApi.getSecret).mockRejectedValue(new Error("Network error"));

    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });

    // Header button should now say "Retry" instead of "Load values"
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /load values/i })).not.toBeInTheDocument();
  });

  it("displays error message on load failure", async () => {
    vi.mocked(secretsApi.getSecret).mockRejectedValue(new Error("Network error"));

    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText(/network error/i)).toBeInTheDocument();
    });
  });

  it("renders labels as badges after loading", async () => {
    const user = userEvent.setup();
    renderComponent();

    await user.click(screen.getByRole("button", { name: /load values/i }));

    await waitFor(() => {
      expect(screen.getByText("app=myapp")).toBeInTheDocument();
    });
  });
});
