// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { TooltipProvider } from "@/components/ui/tooltip";
import { EditSecretDialog } from "./EditSecretDialog";
import type { SecretDetail } from "@/types/secret";

const mockMutateAsync = vi.fn();

vi.mock("@/hooks/useSecrets", () => ({
  useUpdateSecret: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
}));

const mockSecret: SecretDetail = {
  name: "db-credentials",
  namespace: "alpha-ns",
  data: {
    username: "admin",
    password: "oldpass",
  },
  createdAt: "2026-03-15T10:00:00Z",
};

const createTestQueryClient = () =>
  new QueryClient({ defaultOptions: { queries: { retry: false } } });

function renderDialog(props: Partial<React.ComponentProps<typeof EditSecretDialog>> = {}) {
  const queryClient = createTestQueryClient();
  const defaultProps = {
    open: true,
    onOpenChange: vi.fn(),
    secret: mockSecret,
    project: "alpha",
    ...props,
  };

  return {
    ...render(
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <MemoryRouter>
            <EditSecretDialog {...defaultProps} />
          </MemoryRouter>
        </TooltipProvider>
      </QueryClientProvider>
    ),
    props: defaultProps,
  };
}

describe("EditSecretDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders with existing key names from secret", () => {
    renderDialog();

    expect(screen.getByText("Edit Secret")).toBeInTheDocument();
    // Should show existing key names pre-filled
    const keyInputs = screen.getAllByLabelText(/^Key /);
    expect(keyInputs).toHaveLength(2);
    expect(keyInputs[0]).toHaveValue("username");
    expect(keyInputs[1]).toHaveValue("password");
  });

  it("submits update with new values", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({
      name: "db-credentials",
      namespace: "alpha-ns",
      keys: ["username", "password"],
      createdAt: "2026-03-15T10:00:00Z",
    });

    const { props } = renderDialog();

    // Fill in new value for username
    const valueInputs = screen.getAllByLabelText(/^Value /);
    await user.type(valueInputs[0], "newadmin");

    // Submit
    await user.click(screen.getByText("Update"));

    // Only keys with non-empty values are sent (partial update)
    expect(mockMutateAsync).toHaveBeenCalledWith({
      name: "db-credentials",
      project: "alpha",
      namespace: "alpha-ns",
      data: {
        username: "newadmin",
      },
    });
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows validation error when no values provided", async () => {
    const user = userEvent.setup();

    renderDialog();

    // Submit without typing any values (all keys have empty values)
    await user.click(screen.getByText("Update"));

    expect(screen.getByText("At least one key must have a new value")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("only sends keys with non-empty values (partial update)", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({
      name: "db-credentials",
      namespace: "alpha-ns",
      keys: ["username", "password"],
      createdAt: "2026-03-15T10:00:00Z",
    });

    const { props } = renderDialog();

    // Only fill in the password value, leave username empty
    const valueInputs = screen.getAllByLabelText(/^Value /);
    await user.type(valueInputs[1], "newpass");

    await user.click(screen.getByText("Update"));

    // Only the key with a value should be sent
    expect(mockMutateAsync).toHaveBeenCalledWith({
      name: "db-credentials",
      project: "alpha",
      namespace: "alpha-ns",
      data: {
        password: "newpass",
      },
    });
    expect(props.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("allows adding a new key-value pair", async () => {
    const user = userEvent.setup();
    mockMutateAsync.mockResolvedValueOnce({
      name: "db-credentials",
      namespace: "alpha-ns",
      keys: ["username", "password", "host"],
      createdAt: "2026-03-15T10:00:00Z",
    });

    renderDialog();

    // Click "Add Key-Value Pair"
    await user.click(screen.getByText("Add Key-Value Pair"));

    // Should now have 3 key inputs
    const keyInputs = screen.getAllByLabelText(/^Key /);
    expect(keyInputs).toHaveLength(3);

    // Fill in the new key and value
    await user.type(keyInputs[2], "host");
    const valueInputs = screen.getAllByLabelText(/^Value /);
    await user.type(valueInputs[2], "db.example.com");

    await user.click(screen.getByText("Update"));

    expect(mockMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        data: expect.objectContaining({ host: "db.example.com" }),
      })
    );
  });

  it("allows removing an existing key row", async () => {
    const user = userEvent.setup();

    renderDialog();

    // Should start with 2 key rows
    expect(screen.getAllByLabelText(/^Key /)).toHaveLength(2);

    // Click remove on the first row
    const removeButtons = screen.getAllByLabelText("Remove row");
    await user.click(removeButtons[0]);

    // Should now have 1 key row
    expect(screen.getAllByLabelText(/^Key /)).toHaveLength(1);
  });
});
