// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { createContext, useContext } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CreateModelDialog } from "./CreateModelDialog";
import * as agentsApi from "@/api/agents";

vi.mock("@/api/agents");
vi.mock("@/hooks/useProjects", () => ({
  useProjects: vi.fn(() => ({ data: { items: [{ name: "alpha" }] }, isLoading: false })),
}));
vi.mock("@/hooks/useNamespaces", () => ({
  useProjectNamespaces: vi.fn(() => ({
    data: { namespaces: ["alpha-apps"] },
    isLoading: false,
    isError: false,
  })),
}));
vi.mock("@/hooks/useAuth", () => ({ useCurrentProject: vi.fn(() => "") }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

// Mock Radix Dialog to avoid portal issues (mirrors CreateSecretDialog test).
vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children, open }: { children: React.ReactNode; open: boolean }) =>
    open ? <div data-testid="dialog">{children}</div> : null,
  DialogContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogHeader: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DialogTitle: ({ children }: { children: React.ReactNode }) => <h2>{children}</h2>,
  DialogDescription: ({ children }: { children: React.ReactNode }) => <p>{children}</p>,
  DialogFooter: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}));

// Mock Radix Select via a context so each Select keeps its own onValueChange.
const SelectMockCtx = createContext<((value: string) => void) | undefined>(undefined);
vi.mock("@/components/ui/select", () => ({
  Select: ({
    children,
    onValueChange,
  }: {
    children: React.ReactNode;
    value: string;
    onValueChange: (value: string) => void;
  }) => <SelectMockCtx.Provider value={onValueChange}>{children}</SelectMockCtx.Provider>,
  SelectTrigger: ({ children, id }: { children: React.ReactNode; id?: string }) => (
    <button role="combobox" id={id}>
      {children}
    </button>
  ),
  SelectValue: ({ children }: { children?: React.ReactNode }) => <span>{children}</span>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => {
    const onValueChange = useContext(SelectMockCtx);
    return (
      <div role="option" data-testid={`select-option-${value}`} onClick={() => onValueChange?.(value)}>
        {children}
      </div>
    );
  },
}));

function renderDialog() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <CreateModelDialog open onOpenChange={() => {}} />
    </QueryClientProvider>
  );
}

describe("CreateModelDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the API key field as a password input", () => {
    renderDialog();
    const apiKey = screen.getByLabelText("API key");
    expect(apiKey).toHaveAttribute("type", "password");
  });

  it("submits a valid form to createModel with the entered fields", async () => {
    vi.mocked(agentsApi.createModel).mockResolvedValue({
      name: "my-model",
      namespace: "alpha-apps",
      provider: "OpenAI",
      model: "gpt-4o",
    });

    renderDialog();

    fireEvent.click(screen.getByTestId("select-option-alpha")); // project
    fireEvent.click(screen.getByTestId("select-option-alpha-apps")); // namespace
    await userEvent.type(screen.getByLabelText("Name"), "my-model");
    await userEvent.type(screen.getByLabelText("API key"), "sk-secret");
    // provider/model default to OpenAI/gpt-4o.

    fireEvent.click(screen.getByRole("button", { name: /^create$/i }));

    await waitFor(() =>
      expect(vi.mocked(agentsApi.createModel)).toHaveBeenCalledWith({
        name: "my-model",
        provider: "OpenAI",
        model: "gpt-4o",
        namespace: "alpha-apps",
        apiKey: "sk-secret",
      })
    );
  });

  it("blocks submission and does not call createModel when required fields are missing", async () => {
    renderDialog();

    fireEvent.click(screen.getByRole("button", { name: /^create$/i }));

    expect(await screen.findByText("Project is required")).toBeInTheDocument();
    expect(vi.mocked(agentsApi.createModel)).not.toHaveBeenCalled();
  });
});
