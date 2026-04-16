// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { usePreferencesStore } from "./preferencesStore";

// Mock the API
vi.mock("@/api/preferences", () => ({
  getPreferences: vi.fn().mockResolvedValue({
    favoriteRgds: ["saved-fav"],
    recentRgds: ["saved-recent"],
  }),
  putPreferences: vi.fn().mockResolvedValue({
    favoriteRgds: [],
    recentRgds: [],
  }),
}));

// Mock sonner toast
vi.mock("sonner", () => ({
  toast: { error: vi.fn() },
}));

describe("usePreferencesStore", () => {
  beforeEach(() => {
    // Reset store between tests
    usePreferencesStore.setState({
      favoriteRgds: [],
      recentRgds: [],
      isLoading: false,
      isHydrated: false,
    });
  });

  it("starts with empty arrays", () => {
    const state = usePreferencesStore.getState();
    expect(state.favoriteRgds).toEqual([]);
    expect(state.recentRgds).toEqual([]);
    expect(state.isHydrated).toBe(false);
  });

  it("hydrates from API", async () => {
    await usePreferencesStore.getState().hydrate();

    const state = usePreferencesStore.getState();
    expect(state.favoriteRgds).toEqual(["saved-fav"]);
    expect(state.recentRgds).toEqual(["saved-recent"]);
    expect(state.isHydrated).toBe(true);
  });

  it("toggleFavorite adds item when not present", async () => {
    usePreferencesStore.setState({ isHydrated: true, favoriteRgds: [] });

    await usePreferencesStore.getState().toggleFavorite("new-rgd");

    expect(usePreferencesStore.getState().favoriteRgds).toContain("new-rgd");
  });

  it("toggleFavorite removes item when present", async () => {
    usePreferencesStore.setState({
      isHydrated: true,
      favoriteRgds: ["existing-rgd"],
    });

    await usePreferencesStore.getState().toggleFavorite("existing-rgd");

    expect(usePreferencesStore.getState().favoriteRgds).not.toContain("existing-rgd");
  });

  it("toggleFavorite truncates to 10 items", async () => {
    const existing = Array.from({ length: 10 }, (_, i) => `rgd-${i}`);
    usePreferencesStore.setState({ isHydrated: true, favoriteRgds: existing });

    await usePreferencesStore.getState().toggleFavorite("rgd-new");

    const favorites = usePreferencesStore.getState().favoriteRgds;
    expect(favorites).toHaveLength(10);
    expect(favorites[0]).toBe("rgd-new");
  });

  it("toggleFavorite reverts on API error and shows toast", async () => {
    const { putPreferences } = await import("@/api/preferences");
    (putPreferences as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error("fail"));
    const { toast } = await import("sonner");

    usePreferencesStore.setState({ isHydrated: true, favoriteRgds: [] });

    await usePreferencesStore.getState().toggleFavorite("fail-rgd");

    // Should have reverted to empty
    expect(usePreferencesStore.getState().favoriteRgds).toEqual([]);
    expect(toast.error).toHaveBeenCalledWith("Failed to update favorite");
  });

  it("addRecent adds to front and deduplicates", () => {
    usePreferencesStore.setState({
      isHydrated: true,
      recentRgds: ["a", "b", "c"],
    });

    usePreferencesStore.getState().addRecent("b");

    const recents = usePreferencesStore.getState().recentRgds;
    expect(recents[0]).toBe("b");
    expect(recents).toEqual(["b", "a", "c"]);
  });

  it("addRecent truncates to 20 items", () => {
    const existing = Array.from({ length: 20 }, (_, i) => `rgd-${i}`);
    usePreferencesStore.setState({ isHydrated: true, recentRgds: existing });

    usePreferencesStore.getState().addRecent("rgd-new");

    const recents = usePreferencesStore.getState().recentRgds;
    expect(recents).toHaveLength(20);
    expect(recents[0]).toBe("rgd-new");
  });
});
