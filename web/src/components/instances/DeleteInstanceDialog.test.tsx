// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { DeleteInstanceDialog } from "./DeleteInstanceDialog";
import type { Instance } from "@/types/rgd";

function createTestInstance(overrides: Partial<Instance> = {}): Instance {
  return {
    name: "my-test-instance",
    namespace: "default",
    rgdName: "my-rgd",
    rgdNamespace: "default",
    apiVersion: "example.com/v1",
    kind: "AKSCluster",
    health: "Healthy",
    conditions: [],
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    uid: "test-uid",
    labels: {},
    ...overrides,
  };
}

describe("DeleteInstanceDialog", () => {
  let onConfirm: ReturnType<typeof vi.fn>;
  let onCancel: ReturnType<typeof vi.fn>;
  let instance: Instance;

  beforeEach(() => {
    onConfirm = vi.fn().mockResolvedValue(undefined);
    onCancel = vi.fn();
    instance = createTestInstance();
  });

  function renderDialog(overrides: Record<string, unknown> = {}) {
    return render(
      <DeleteInstanceDialog
        instance={instance}
        isOpen={true}
        onConfirm={onConfirm}
        onCancel={onCancel}
        {...overrides}
      />
    );
  }

  it("renders title with instance name", () => {
    renderDialog();
    expect(
      screen.getByText(`Delete ${instance.name}?`)
    ).toBeInTheDocument();
  });

  it("renders consequence description", () => {
    renderDialog();
    expect(
      screen.getByText(/All resources managed by this instance will be deleted/)
    ).toBeInTheDocument();
  });

  it("renders type-to-confirm label with instance name", () => {
    renderDialog();
    expect(screen.getByText(instance.name, { selector: "code" })).toBeInTheDocument();
  });

  it("has Delete button disabled by default", () => {
    renderDialog();
    const deleteButton = screen.getByTestId("confirm-delete-button");
    expect(deleteButton).toBeDisabled();
  });

  it("keeps Delete button disabled when partial name typed", () => {
    renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: "my-test" } });
    expect(screen.getByTestId("confirm-delete-button")).toBeDisabled();
  });

  it("enables Delete button when exact name matches", () => {
    renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: instance.name } });
    expect(screen.getByTestId("confirm-delete-button")).not.toBeDisabled();
  });

  it("calls onConfirm when Delete clicked with valid name", async () => {
    renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: instance.name } });
    fireEvent.click(screen.getByTestId("confirm-delete-button"));
    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce();
    });
  });

  it("does not call onConfirm when Delete clicked with invalid name", () => {
    renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: "wrong-name" } });
    fireEvent.click(screen.getByTestId("confirm-delete-button"));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("calls onCancel when Cancel clicked", () => {
    renderDialog();
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();
  });

  it("renders error box when error prop is provided", () => {
    renderDialog({ error: new Error("Network failure") });
    expect(screen.getByText("Network failure")).toBeInTheDocument();
  });

  it("does not render error box when error is null", () => {
    renderDialog({ error: null });
    expect(screen.queryByText("Failed to delete instance")).not.toBeInTheDocument();
  });

  it("shows spinner and disables inputs when isDeleting is true", () => {
    renderDialog({ isDeleting: true });
    const input = screen.getByTestId("confirm-name-input");
    expect(input).toBeDisabled();
    expect(screen.getByRole("button", { name: /cancel/i })).toBeDisabled();
  });

  it("resets confirm input on cancel", () => {
    const { rerender } = renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: instance.name } });
    expect(input).toHaveValue(instance.name);
    fireEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(onCancel).toHaveBeenCalledOnce();

    // Re-open dialog and verify input was reset
    rerender(
      <DeleteInstanceDialog
        instance={instance}
        isOpen={true}
        onConfirm={onConfirm}
        onCancel={onCancel}
      />
    );
    expect(screen.getByTestId("confirm-name-input")).toHaveValue("");
  });

  it("calls onCancel when dialog is dismissed via onOpenChange", () => {
    // When AlertDialog's onOpenChange fires with false, handleCancel is called
    renderDialog();
    // Pressing Escape triggers onOpenChange(false) in Radix AlertDialog
    fireEvent.keyDown(document.activeElement || document.body, {
      key: "Escape",
    });
    expect(onCancel).toHaveBeenCalled();
  });

  it("keeps Delete button disabled when name differs in case", () => {
    renderDialog();
    const input = screen.getByTestId("confirm-name-input");
    fireEvent.change(input, { target: { value: instance.name.toUpperCase() } });
    expect(screen.getByTestId("confirm-delete-button")).toBeDisabled();
  });

  it("does not render when isOpen is false", () => {
    renderDialog({ isOpen: false });
    expect(screen.queryByText(`Delete ${instance.name}?`)).not.toBeInTheDocument();
  });
});
