// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { lazy, Suspense } from "react";
import { RouteErrorBoundary } from "./route-error-boundary";

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate, useLocation: () => ({ pathname: "/test-route" }) };
});

// Component that throws on demand
function ThrowingComponent({ shouldThrow }: { shouldThrow: boolean }) {
  if (shouldThrow) {
    throw new Error("Test error message");
  }
  return <div>Child content</div>;
}

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  mockNavigate.mockClear();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("RouteErrorBoundary", () => {
  it("renders children when no error occurs", () => {
    render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={false} />
      </RouteErrorBoundary>
    );
    expect(screen.getByText("Child content")).toBeInTheDocument();
  });

  it("catches errors and renders inline error card", () => {
    render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );
    expect(screen.getByTestId("route-error-boundary")).toBeInTheDocument();
    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
    expect(screen.getByText("Test error message")).toBeInTheDocument();
    expect(
      screen.getByText(/other parts of the app are unaffected/i)
    ).toBeInTheDocument();
  });

  it("renders custom fallback when provided", () => {
    render(
      <RouteErrorBoundary fallback={<div>Custom fallback</div>}>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );
    expect(screen.getByText("Custom fallback")).toBeInTheDocument();
    expect(screen.queryByTestId("route-error-boundary")).not.toBeInTheDocument();
  });

  it("resets error state when Retry is clicked", () => {
    const { rerender } = render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );

    // Error card should be visible
    expect(screen.getByTestId("route-error-boundary")).toBeInTheDocument();

    // Re-render with non-throwing child before clicking retry
    rerender(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={false} />
      </RouteErrorBoundary>
    );

    // Click Retry to reset error state
    fireEvent.click(screen.getByRole("button", { name: /retry/i }));

    // Children should render again
    expect(screen.getByText("Child content")).toBeInTheDocument();
    expect(screen.queryByTestId("route-error-boundary")).not.toBeInTheDocument();
  });

  it("calls navigate(-1) when Go Back is clicked and history exists", () => {
    const originalLength = window.history.length;
    Object.defineProperty(window.history, "length", { value: 3, writable: true, configurable: true });

    render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );

    fireEvent.click(screen.getByRole("button", { name: /go back/i }));
    expect(mockNavigate).toHaveBeenCalledWith(-1);

    Object.defineProperty(window.history, "length", { value: originalLength, writable: true, configurable: true });
  });

  it("navigates to /instances fallback when no history available", () => {
    const originalLength = window.history.length;
    Object.defineProperty(window.history, "length", { value: 1, writable: true, configurable: true });

    render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );

    fireEvent.click(screen.getByRole("button", { name: /go back/i }));
    expect(mockNavigate).toHaveBeenCalledWith("/instances", { replace: true });

    Object.defineProperty(window.history, "length", { value: originalLength, writable: true, configurable: true });
  });

  it("shows Go Back and Retry buttons in error state", () => {
    render(
      <RouteErrorBoundary>
        <ThrowingComponent shouldThrow={true} />
      </RouteErrorBoundary>
    );

    expect(screen.getByRole("button", { name: /go back/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /retry/i })).toBeInTheDocument();
  });

  it("catches lazy-load chunk failures", async () => {
    const LazyFailing = lazy(() => Promise.reject(new Error("Failed to fetch dynamically imported module")));

    render(
      <RouteErrorBoundary>
        <Suspense fallback={<div>Loading...</div>}>
          <LazyFailing />
        </Suspense>
      </RouteErrorBoundary>
    );

    await waitFor(() => {
      expect(screen.getByTestId("route-error-boundary")).toBeInTheDocument();
    });
    expect(screen.getByText(/failed to fetch dynamically imported module/i)).toBeInTheDocument();
  });
});
