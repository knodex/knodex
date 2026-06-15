// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useUsers, useReclaimUser } from "./useUsers";
import * as usersApi from "@/api/users";
import type { User, UsersListResponse } from "@/types/user";

// Mock the users API
vi.mock("@/api/users");

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function makeUser(overrides: Partial<User> = {}): User {
  return {
    id: "u1",
    email: "alice@test.local",
    displayName: "Alice",
    state: "active",
    isInactive: false,
    applicationRole: "member",
    firstSeenAt: "2026-01-01T10:00:00Z",
    lastSeenAt: "2026-06-01T10:00:00Z",
    federatedIdentities: [
      {
        issuer: "https://idp.test.local",
        sub: "alice-sub",
        providerKind: "oidc",
        sourceKind: "oidc_jit",
        createdAt: "2026-01-01T10:00:00Z",
        updatedAt: "2026-06-01T10:00:00Z",
      },
    ],
    ...overrides,
  };
}

describe("useUsers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches and returns the roster on success", async () => {
    const page: UsersListResponse = { users: [makeUser()] };
    vi.mocked(usersApi.listUsers).mockResolvedValue(page);

    const { result } = renderHook(() => useUsers(), {
      wrapper: createWrapper(),
    });

    expect(result.current.isLoading).toBe(true);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.pages[0].users).toHaveLength(1);
    expect(result.current.data?.pages[0].users[0].email).toBe(
      "alice@test.local",
    );
    expect(usersApi.listUsers).toHaveBeenCalledWith({
      limit: undefined,
      pageToken: undefined,
    });
  });

  it("does not retry on a 403", async () => {
    const error = Object.assign(new Error("Forbidden"), {
      response: { status: 403 },
    });
    vi.mocked(usersApi.listUsers).mockRejectedValue(error);

    const { result } = renderHook(() => useUsers(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    // 403 short-circuits the retry predicate — called exactly once.
    expect(usersApi.listUsers).toHaveBeenCalledTimes(1);
  });

  it("appends the next page on fetchNextPage (keyset pagination)", async () => {
    const firstPage: UsersListResponse = {
      users: [makeUser({ id: "u1", email: "alice@test.local" })],
      nextPageToken: "cursor-2",
    };
    const secondPage: UsersListResponse = {
      users: [makeUser({ id: "u2", email: "bob@test.local" })],
    };
    vi.mocked(usersApi.listUsers)
      .mockResolvedValueOnce(firstPage)
      .mockResolvedValueOnce(secondPage);

    const { result } = renderHook(() => useUsers(), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.hasNextPage).toBe(true);

    result.current.fetchNextPage();

    await waitFor(() => {
      expect(result.current.data?.pages).toHaveLength(2);
    });

    const allUsers = result.current.data?.pages.flatMap((p) => p.users) ?? [];
    expect(allUsers.map((u) => u.id)).toEqual(["u1", "u2"]);
    // Second call carries the cursor from the first page.
    expect(usersApi.listUsers).toHaveBeenLastCalledWith({
      limit: undefined,
      pageToken: "cursor-2",
    });
    expect(result.current.hasNextPage).toBe(false);
  });
});

/**
 * Story 16.2 reclaim mutation. The 404-as-success behavior lives in the HOOK
 * (the cleaner of the two options Task 2 offered), so it's exercised here
 * against a mocked `deleteUser` rather than in the component test.
 */
function makeWrapperWithSpy() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return { wrapper, invalidateSpy };
}

/** Top-level query keys across all invalidateQueries calls. */
function invalidatedKeys(
  invalidateSpy: ReturnType<typeof vi.spyOn>,
): unknown[] {
  return invalidateSpy.mock.calls.map(
    (c) => (c[0] as { queryKey: unknown[] }).queryKey[0],
  );
}

describe("useReclaimUser", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("invalidates BOTH [users] and [license] on success (AC #4)", async () => {
    vi.mocked(usersApi.deleteUser).mockResolvedValue({
      id: "u1",
      state: "removed",
      note: "Seat reclaimed. Permanent exclusion requires IdP-side revocation.",
    });
    const { wrapper, invalidateSpy } = makeWrapperWithSpy();
    const { result } = renderHook(() => useReclaimUser(), { wrapper });

    await result.current.mutateAsync("u1");

    expect(usersApi.deleteUser).toHaveBeenCalledWith("u1");
    const keys = invalidatedKeys(invalidateSpy);
    expect(keys).toContain("users");
    expect(keys).toContain("license");
  });

  it("treats a 404 as success/no-op — resolves, no throw, still invalidates (AC #4)", async () => {
    const err = Object.assign(new Error("not found"), {
      response: { status: 404 },
    });
    vi.mocked(usersApi.deleteUser).mockRejectedValue(err);
    const { wrapper, invalidateSpy } = makeWrapperWithSpy();
    const { result } = renderHook(() => useReclaimUser(), { wrapper });

    await expect(result.current.mutateAsync("u-gone")).resolves.toMatchObject({
      id: "u-gone",
      state: "removed",
    });
    const keys = invalidatedKeys(invalidateSpy);
    expect(keys).toContain("users");
    expect(keys).toContain("license");
  });

  it("propagates non-404 errors to the caller", async () => {
    const err = Object.assign(new Error("boom"), {
      response: { status: 500 },
    });
    vi.mocked(usersApi.deleteUser).mockRejectedValue(err);
    const { wrapper } = makeWrapperWithSpy();
    const { result } = renderHook(() => useReclaimUser(), { wrapper });

    await expect(result.current.mutateAsync("u1")).rejects.toThrow("boom");
  });
});
