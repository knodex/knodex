// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { SecretDetailView } from "./SecretDetailView";
import * as useCanIModule from "@/hooks/useCanI";
import * as _useSecretsModule from "@/hooks/useSecrets";
import type { SecretDetail } from "@/types/secret";

vi.mock("@/hooks/useCanI", () => ({
  useCanI: vi.fn(),
}));

vi.mock("@/hooks/useSecrets", () => ({
  useUpdateSecret: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteSecret: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}));

const mockSecret: SecretDetail = {
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

function renderComponent(secret: SecretDetail = mockSecret) {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <MemoryRouter>
          <SecretDetailView secret={secret} project="alpha" />
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
  });

  it("renders secret data with masked values", () => {
    renderComponent();

    expect(screen.getByText("db-credentials")).toBeInTheDocument();
    expect(screen.getByText("alpha-ns")).toBeInTheDocument();
    expect(screen.getByText("username")).toBeInTheDocument();
    expect(screen.getByText("password")).toBeInTheDocument();
    // Values should be masked by default
    expect(screen.getAllByText("••••••••")).toHaveLength(2);
    expect(screen.queryByText("admin")).not.toBeInTheDocument();
    expect(screen.queryByText("supersecret123")).not.toBeInTheDocument();
  });

  it("shows/hides values when toggle is clicked", async () => {
    const user = userEvent.setup();
    renderComponent();

    // Click "Show username" toggle
    const showButtons = screen.getAllByLabelText(/^Show /);
    expect(showButtons.length).toBe(2);

    await user.click(showButtons[0]);

    // Now the username value should be visible
    expect(screen.getByText("admin")).toBeInTheDocument();
    // Password should still be masked
    expect(screen.queryByText("supersecret123")).not.toBeInTheDocument();
  });

  it("hides Edit button without update permission", () => {
    vi.mocked(useCanIModule.useCanI).mockImplementation((_resource, action) => {
      if (action === "update") return { allowed: false, isLoading: false, isError: false };
      return { allowed: true, isLoading: false, isError: false };
    });

    renderComponent();

    expect(screen.queryByText("Edit")).not.toBeInTheDocument();
    expect(screen.getByText("Delete")).toBeInTheDocument();
  });

  it("hides Delete button without delete permission", () => {
    vi.mocked(useCanIModule.useCanI).mockImplementation((_resource, action) => {
      if (action === "delete") return { allowed: false, isLoading: false, isError: false };
      return { allowed: true, isLoading: false, isError: false };
    });

    renderComponent();

    expect(screen.getByText("Edit")).toBeInTheDocument();
    expect(screen.queryByText("Delete")).not.toBeInTheDocument();
  });

  it("renders labels as badges", () => {
    renderComponent();

    expect(screen.getByText("app=myapp")).toBeInTheDocument();
  });

  it("shows key count in section header", () => {
    renderComponent();

    expect(screen.getByText("Data (2 keys)")).toBeInTheDocument();
  });
});
