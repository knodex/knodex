// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { DeleteSecretDialog } from "./DeleteSecretDialog";
import * as _useSecretsModule from "@/hooks/useSecrets";

const mockMutateAsync = vi.fn();

vi.mock("@/hooks/useSecrets", () => ({
  useDeleteSecret: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
}));

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const createTestQueryClient = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderDialog(props: Partial<React.ComponentProps<typeof DeleteSecretDialog>> = {}) {
  const queryClient = createTestQueryClient();
  const defaultProps = {
    open: true,
    onOpenChange: vi.fn(),
    secretName: "db-credentials",
    secretNamespace: "alpha-ns",
    project: "alpha",
    ...props,
  };

  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <MemoryRouter>
            <DeleteSecretDialog {...defaultProps} />
          </MemoryRouter>
        </TooltipProvider>
      </QueryClientProvider>
    ),
    props: defaultProps,
  };
}

describe("DeleteSecretDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows warning about instance references", () => {
    renderDialog();

    expect(screen.getByRole("button", { name: "Delete Secret" })).toBeInTheDocument();
    expect(screen.getByText(/cannot be undone/)).toBeInTheDocument();
    expect(screen.getByText(/Instances referencing this secret may be affected/)).toBeInTheDocument();
    // Verify cancel button present
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("calls delete mutation and navigates on confirm", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({ deleted: true, warnings: [] });

    const { props } = renderDialog();

    await user.click(screen.getByRole("button", { name: "Delete Secret" }));

    expect(mockMutateAsync).toHaveBeenCalledWith({
      name: "db-credentials",
      project: "alpha",
      namespace: "alpha-ns",
    });
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
    expect(mockNavigate).toHaveBeenCalledWith("/secrets?project=alpha");
  });

  it("shows warnings from API response in dialog after deletion", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({
      deleted: true,
      warnings: ["Referenced by Instance my-app"],
    });

    renderDialog();

    await user.click(screen.getByRole("button", { name: "Delete Secret" }));

    // Warnings must be shown inside the dialog, not just as a toast
    await waitFor(() => {
      expect(screen.getByText("Referenced by Instance my-app")).toBeInTheDocument();
    });
    expect(screen.getByText("Secret Deleted — Warnings")).toBeInTheDocument();
    // OK button dismisses the warnings view and navigates
    expect(screen.getByRole("button", { name: "OK" })).toBeInTheDocument();
  });

  it("does not navigate when navigateOnDelete is false", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({ deleted: true, warnings: [] });

    renderDialog({ navigateOnDelete: false });

    await user.click(screen.getByRole("button", { name: "Delete Secret" }));

    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
