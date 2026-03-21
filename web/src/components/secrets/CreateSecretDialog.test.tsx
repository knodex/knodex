// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CreateSecretDialog } from "./CreateSecretDialog";
import * as useSecretsModule from "@/hooks/useSecrets";
import * as useNamespacesModule from "@/hooks/useNamespaces";

vi.mock("@/hooks/useSecrets", () => ({
  useCreateSecret: vi.fn(),
}));

vi.mock("@/hooks/useNamespaces", () => ({
  useProjectNamespaces: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock Radix UI Dialog to avoid portal issues
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-content">{children}</div>
  ),
  DialogHeader: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-header">{children}</div>
  ),
  DialogTitle: ({ children }: { children: React.ReactNode }) => (
    <h2 data-testid="dialog-title">{children}</h2>
  ),
  DialogDescription: ({ children }: { children: React.ReactNode }) => (
    <p data-testid="dialog-description">{children}</p>
  ),
  DialogFooter: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="dialog-footer">{children}</div>
  ),
}));

// Mock Radix UI Select
let selectOnValueChange: ((value: string) => void) | undefined;
vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    onValueChange,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => {
    selectOnValueChange = onValueChange;
    return <div data-testid="select">{children}</div>;
  },
  SelectTrigger: ({ children }: { children: React.ReactNode }) => (
    <button role="combobox">{children}</button>
  ),
  SelectValue: ({ children }: { children?: React.ReactNode }) => (
    <span>{children}</span>
  ),
  SelectContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="select-content">{children}</div>
  ),
  SelectItem: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => (
    <div
      role="option"
      data-testid={`select-option-${value}`}
      onClick={() => selectOnValueChange?.(value)}
    >
      {children}
    </div>
  ),
}));

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

function renderDialog() {
  const queryClient = createTestQueryClient();
  const onOpenChange = vi.fn();
  const result = render(
    <QueryClientProvider client={queryClient}>
      <CreateSecretDialog
        open={true}
        onOpenChange={onOpenChange}
        project="alpha"
      />
    </QueryClientProvider>
  );
  return { ...result, onOpenChange };
}

describe("CreateSecretDialog", () => {
  const mockMutateAsync = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSecretsModule.useCreateSecret).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useSecretsModule.useCreateSecret>);
    vi.mocked(useNamespacesModule.useProjectNamespaces).mockReturnValue({
      data: { namespaces: ["alpha-ns", "beta-ns"], count: 2 },
      isLoading: false,
    } as unknown as ReturnType<typeof useNamespacesModule.useProjectNamespaces>);
  });

  it("submits correct data", async () => {
    mockMutateAsync.mockResolvedValue({
      name: "my-secret",
      namespace: "alpha-ns",
      keys: ["API_KEY"],
      createdAt: "2026-03-19T00:00:00Z",
    });

    renderDialog();

    // Fill name
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "my-secret" },
    });

    // Select namespace via mock
    fireEvent.click(screen.getByTestId("select-option-alpha-ns"));

    // Fill key-value pair
    fireEvent.change(screen.getByLabelText("Key 1"), {
      target: { value: "API_KEY" },
    });
    fireEvent.change(screen.getByLabelText("Value 1"), {
      target: { value: "secret-123" },
    });

    // Submit
    fireEvent.click(screen.getByText("Create"));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        project: "alpha",
        name: "my-secret",
        namespace: "alpha-ns",
        data: { API_KEY: "secret-123" },
      });
    });
  });

  it("shows validation errors when fields are empty", async () => {
    renderDialog();

    fireEvent.click(screen.getByText("Create"));

    await waitFor(() => {
      expect(screen.getByText("Name is required")).toBeInTheDocument();
      expect(screen.getByText("Namespace is required")).toBeInTheDocument();
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("rejects invalid K8s secret names", async () => {
    renderDialog();

    // Enter an invalid name (uppercase, spaces)
    fireEvent.change(screen.getByLabelText("Name"), {
      target: { value: "MY SECRET!" },
    });

    fireEvent.click(screen.getByText("Create"));

    await waitFor(() => {
      expect(
        screen.getByText("Name must be lowercase alphanumeric, hyphens, or dots (e.g. my-secret)")
      ).toBeInTheDocument();
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });
});
