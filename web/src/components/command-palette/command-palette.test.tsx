// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { CommandPalette } from "./command-palette";

// cmdk requires constructable ResizeObserver and scrollIntoView
beforeAll(() => {
  globalThis.ResizeObserver = class {
    observe = vi.fn();
    unobserve = vi.fn();
    disconnect = vi.fn();
  } as unknown as typeof ResizeObserver;

  // cmdk calls scrollIntoView which jsdom doesn't implement
  Element.prototype.scrollIntoView = vi.fn();
});

// Mock the search API
vi.mock("@/api/search", () => ({
  searchAll: vi.fn().mockResolvedValue({
    results: {
      rgds: [
        { name: "postgres-db", displayName: "PostgreSQL", category: "database", description: "Managed PostgreSQL" },
      ],
      instances: [
        { name: "my-instance", project: "alpha", namespace: "default", status: "Healthy", kind: "MyDB" },
      ],
      projects: [
        { name: "alpha", description: "Alpha project" },
      ],
    },
    query: "test",
    totalCount: 3,
  }),
}));

// Mock useNavigate
const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>{children}</MemoryRouter>
      </QueryClientProvider>
    );
  };
}

describe("CommandPalette", () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    localStorage.clear();
  });

  it("renders when open=true", () => {
    render(<CommandPalette open={true} onOpenChange={vi.fn()} />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByPlaceholderText("Search RGDs, instances, projects...")).toBeInTheDocument();
  });

  it("does not render content when open=false", () => {
    render(<CommandPalette open={false} onOpenChange={vi.fn()} />, {
      wrapper: createWrapper(),
    });

    expect(screen.queryByPlaceholderText("Search RGDs, instances, projects...")).not.toBeInTheDocument();
  });

  it("shows Navigate group with static items when open with empty query", () => {
    render(<CommandPalette open={true} onOpenChange={vi.fn()} />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Instances")).toBeInTheDocument();
    expect(screen.getByText("Catalog")).toBeInTheDocument();
    expect(screen.getByText("Projects")).toBeInTheDocument();
    expect(screen.getByText("Secrets")).toBeInTheDocument();
    expect(screen.getByText("Settings")).toBeInTheDocument();
  });

  it("shows Recent group when recent items exist in localStorage", () => {
    const recent = [
      { id: "nav-instances", label: "Instances", href: "/instances", timestamp: Date.now() },
    ];
    localStorage.setItem("command-palette-recent", JSON.stringify(recent));

    render(<CommandPalette open={true} onOpenChange={vi.fn()} />, {
      wrapper: createWrapper(),
    });

    // Should show "Recent" heading
    expect(screen.getByText("Recent")).toBeInTheDocument();
  });

  it("calls onOpenChange(false) when Escape is pressed", () => {
    const onOpenChange = vi.fn();
    render(<CommandPalette open={true} onOpenChange={onOpenChange} />, {
      wrapper: createWrapper(),
    });

    fireEvent.keyDown(document, { key: "Escape" });

    // Radix Dialog handles Escape and calls onOpenChange(false)
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("navigates and closes when a Navigate item is selected", async () => {
    const onOpenChange = vi.fn();
    render(<CommandPalette open={true} onOpenChange={onOpenChange} />, {
      wrapper: createWrapper(),
    });

    // Click on Catalog
    fireEvent.click(screen.getByText("Catalog"));

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith("/catalog");
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it("saves selected item to recent in localStorage", async () => {
    const onOpenChange = vi.fn();
    render(<CommandPalette open={true} onOpenChange={onOpenChange} />, {
      wrapper: createWrapper(),
    });

    fireEvent.click(screen.getByText("Catalog"));

    await waitFor(() => {
      const recent = JSON.parse(localStorage.getItem("command-palette-recent") || "[]");
      expect(recent).toHaveLength(1);
      expect(recent[0].label).toBe("Catalog");
    });
  });

  it("has accessible dialog title", () => {
    render(<CommandPalette open={true} onOpenChange={vi.fn()} />, {
      wrapper: createWrapper(),
    });

    // CommandDialog includes DialogTitle with sr-only class
    expect(screen.getByText("Command Menu")).toBeInTheDocument();
  });

  it("includes aria-live region for results announcement", () => {
    render(
      <CommandPalette open={true} onOpenChange={vi.fn()} />,
      { wrapper: createWrapper() }
    );

    // aria-live is rendered inside a Radix Dialog portal (in document.body, not container)
    const liveRegion = document.querySelector("[aria-live='polite']");
    expect(liveRegion).toBeInTheDocument();
  });
});
