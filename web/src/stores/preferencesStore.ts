// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from "zustand";
import { toast } from "sonner";
import { getPreferences, putPreferences } from "@/api/preferences";

const MAX_FAVORITES = 10;
const MAX_RECENT = 20;

interface PreferencesState {
  favoriteRgds: string[];
  recentRgds: string[];
  isLoading: boolean;
  isHydrated: boolean;
  hydrate: () => Promise<void>;
  toggleFavorite: (rgdName: string) => Promise<void>;
  addRecent: (rgdName: string) => void;
}

export const usePreferencesStore = create<PreferencesState>((set, get) => ({
  favoriteRgds: [],
  recentRgds: [],
  isLoading: false,
  isHydrated: false,

  hydrate: async () => {
    if (get().isHydrated || get().isLoading) return;
    set({ isLoading: true });
    try {
      const prefs = await getPreferences();
      set({
        favoriteRgds: prefs.favoriteRgds ?? [],
        recentRgds: prefs.recentRgds ?? [],
        isHydrated: true,
        isLoading: false,
      });
    } catch {
      set({ isLoading: false, isHydrated: true });
    }
  },

  toggleFavorite: async (rgdName: string) => {
    const { favoriteRgds, recentRgds } = get();
    const snapshot = { favoriteRgds: [...favoriteRgds], recentRgds: [...recentRgds] };

    const isFav = favoriteRgds.includes(rgdName);
    const newFavorites = isFav
      ? favoriteRgds.filter((id) => id !== rgdName)
      : [rgdName, ...favoriteRgds].slice(0, MAX_FAVORITES);

    // Optimistic update
    set({ favoriteRgds: newFavorites });

    try {
      await putPreferences({
        favoriteRgds: newFavorites,
        recentRgds: get().recentRgds,
      });
    } catch {
      // Revert on error
      set({ favoriteRgds: snapshot.favoriteRgds });
      toast.error("Failed to update favorite");
    }
  },

  addRecent: (rgdName: string) => {
    const { recentRgds } = get();
    const deduplicated = recentRgds.filter((id) => id !== rgdName);
    const newRecent = [rgdName, ...deduplicated].slice(0, MAX_RECENT);
    set({ recentRgds: newRecent });

    // Fire-and-forget persist
    putPreferences({
      favoriteRgds: get().favoriteRgds,
      recentRgds: newRecent,
    }).catch(() => {
      // Silent failure for recents — not critical
    });
  },
}));
