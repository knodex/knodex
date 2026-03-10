// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import * as useAuditModule from "@/hooks/useAudit";
import { AuditConfigForm } from "./AuditConfigForm";

// Mock the audit hooks
vi.mock("@/hooks/useAudit", () => ({
  useAuditConfig: vi.fn(),
  useUpdateAuditConfig: vi.fn(),
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

const mockConfig = {
  enabled: true,
  retentionDays: 90,
  maxStreamLength: 100000,
  excludeActions: ["get"],
  excludeResources: ["health"],
};

const mockMutateAsync = vi.fn();

describe("AuditConfigForm", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMutateAsync.mockResolvedValue(mockConfig);

    vi.mocked(useAuditModule.useAuditConfig).mockReturnValue({
      data: mockConfig,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useAuditModule.useAuditConfig>);

    vi.mocked(useAuditModule.useUpdateAuditConfig).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: false,
    } as unknown as ReturnType<typeof useAuditModule.useUpdateAuditConfig>);
  });

  it("renders the configuration form with current values", () => {
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    expect(screen.getByText("Audit Configuration")).toBeInTheDocument();
    expect(screen.getByText("Enable Audit Trail")).toBeInTheDocument();
    expect(screen.getByLabelText("Retention Days")).toHaveValue(90);
  });

  it("does not show advanced fields (maxStreamLength, excludeActions, excludeResources)", () => {
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    expect(screen.queryByLabelText("Max Stream Length")).not.toBeInTheDocument();
    expect(screen.queryByText("Exclude Actions")).not.toBeInTheDocument();
    expect(screen.queryByText("Exclude Resources")).not.toBeInTheDocument();
  });

  it("renders the enabled checkbox checked when enabled", () => {
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeChecked();
  });

  it("shows loading skeleton when config is loading", () => {
    vi.mocked(useAuditModule.useAuditConfig).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
    } as ReturnType<typeof useAuditModule.useAuditConfig>);

    const { container } = render(<AuditConfigForm />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Audit Configuration")).toBeInTheDocument();
    const skeletons = container.querySelectorAll('[class*="animate-pulse"]');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("disables save button when no changes made", () => {
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    expect(saveButton).toBeDisabled();
  });

  it("enables save button when form values change", async () => {
    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "30");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    expect(saveButton).not.toBeDisabled();
  });

  it("submits form with updated values", async () => {
    const { toast } = await import("sonner");
    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "30");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          retentionDays: 30,
          enabled: true,
          maxStreamLength: 100000,
          excludeActions: ["get"],
          excludeResources: ["health"],
        })
      );
    });

    expect(toast.success).toHaveBeenCalledWith("Audit configuration saved");
  });

  it("shows error toast on save failure", async () => {
    const { toast } = await import("sonner");
    mockMutateAsync.mockRejectedValue(new Error("Network error"));

    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "30");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Network error");
    });
  });

  it("validates retentionDays minimum", async () => {
    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "0");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(screen.getByText("Minimum 1 day")).toBeInTheDocument();
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("validates retentionDays maximum", async () => {
    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "9999");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(
        screen.getByText("Maximum 3650 days (10 years)")
      ).toBeInTheDocument();
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("disables all inputs and save button during pending mutation", () => {
    vi.mocked(useAuditModule.useUpdateAuditConfig).mockReturnValue({
      mutateAsync: mockMutateAsync,
      isPending: true,
    } as unknown as ReturnType<typeof useAuditModule.useUpdateAuditConfig>);

    render(<AuditConfigForm />, { wrapper: createWrapper() });

    expect(screen.getByRole("checkbox")).toBeDisabled();
    expect(screen.getByLabelText("Retention Days")).toBeDisabled();
    expect(
      screen.getByRole("button", { name: /save configuration/i })
    ).toBeDisabled();
  });

  it("shows error banner when config fetch fails", () => {
    vi.mocked(useAuditModule.useAuditConfig).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Server error"),
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useAuditModule.useAuditConfig>);

    render(<AuditConfigForm />, { wrapper: createWrapper() });

    expect(
      screen.getByText("Failed to load audit configuration")
    ).toBeInTheDocument();
    // Form fields should NOT be rendered
    expect(screen.queryByLabelText("Retention Days")).not.toBeInTheDocument();
  });

  it("calls refetch when retry button is clicked on error state", async () => {
    const mockRefetch = vi.fn();
    vi.mocked(useAuditModule.useAuditConfig).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("Server error"),
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useAuditModule.useAuditConfig>);

    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retryButton = screen.getByRole("button", { name: /retry/i });
    await user.click(retryButton);

    expect(mockRefetch).toHaveBeenCalledOnce();
  });

  it("submits with enabled: false when checkbox is unchecked", async () => {
    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    // Uncheck the "Enable Audit Trail" checkbox
    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeChecked();
    await user.click(checkbox);
    expect(checkbox).not.toBeChecked();

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          enabled: false,
          retentionDays: 90,
        })
      );
    });
  });

  it("shows fallback error toast for non-Error rejection", async () => {
    const { toast } = await import("sonner");
    mockMutateAsync.mockRejectedValue("unexpected string error");

    const user = userEvent.setup();
    render(<AuditConfigForm />, { wrapper: createWrapper() });

    const retentionInput = screen.getByLabelText("Retention Days");
    await user.clear(retentionInput);
    await user.type(retentionInput, "30");

    const saveButton = screen.getByRole("button", {
      name: /save configuration/i,
    });
    await user.click(saveButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "Failed to save configuration"
      );
    });
  });
});
