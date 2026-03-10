// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ErrorState } from "./ErrorState";

describe("ErrorState", () => {
  it("renders error message (AC-SHARED-04)", () => {
    render(<ErrorState message="Failed to load data" />);

    expect(screen.getByText("Failed to load data")).toBeInTheDocument();
  });

  it("renders error details when provided (AC-SHARED-04)", () => {
    render(
      <ErrorState
        message="Failed to load constraints"
        details="Network connection refused"
      />
    );

    expect(screen.getByText("Failed to load constraints")).toBeInTheDocument();
    expect(screen.getByText("Network connection refused")).toBeInTheDocument();
  });

  it("renders retry button when onRetry is provided (AC-SHARED-04)", () => {
    const onRetry = vi.fn();
    render(
      <ErrorState message="Failed to fetch" onRetry={onRetry} />
    );

    // Button says "Try Again"
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("calls onRetry when retry button is clicked", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    render(
      <ErrorState message="Failed to fetch" onRetry={onRetry} />
    );

    const retryButton = screen.getByRole("button", { name: /try again/i });
    await user.click(retryButton);

    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("shows loading state when isRetrying is true", () => {
    render(
      <ErrorState
        message="Failed to fetch"
        onRetry={vi.fn()}
        isRetrying={true}
      />
    );

    // Should show "Retrying..." text and button should be disabled
    const retryButton = screen.getByRole("button", { name: /retrying/i });
    expect(retryButton).toBeDisabled();
  });

  it("does not render retry button when onRetry is not provided", () => {
    render(<ErrorState message="Error occurred" />);

    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders error icon", () => {
    const { container } = render(<ErrorState message="Something went wrong" />);

    // The component should include an error icon (AlertTriangle)
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    // Icon is rendered as SVG
    expect(container.querySelector("svg")).toBeInTheDocument();
  });

  it("renders without details", () => {
    render(<ErrorState message="Error" onRetry={vi.fn()} />);

    expect(screen.getByText("Error")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /try again/i })).toBeInTheDocument();
  });

  it("handles long error messages", () => {
    const longMessage = "This is a very long error message that explains in detail what went wrong during the operation and provides context about the failure";
    render(<ErrorState message={longMessage} />);

    expect(screen.getByText(longMessage)).toBeInTheDocument();
  });

  it("handles long error details", () => {
    const longDetails = "Error: ECONNREFUSED 127.0.0.1:8080 - Unable to connect to the backend service. This may be due to the service being unavailable or network connectivity issues.";
    render(
      <ErrorState
        message="Connection Failed"
        details={longDetails}
      />
    );

    expect(screen.getByText(longDetails)).toBeInTheDocument();
  });

  it("enables retry button when not retrying", () => {
    render(
      <ErrorState
        message="Failed"
        onRetry={vi.fn()}
        isRetrying={false}
      />
    );

    const retryButton = screen.getByRole("button", { name: /try again/i });
    expect(retryButton).not.toBeDisabled();
  });

  it("applies custom className", () => {
    const { container } = render(
      <ErrorState message="Error" className="custom-class" />
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });
});
