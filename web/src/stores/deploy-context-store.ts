// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

interface StackProgress {
  current: number;
  total: number;
}

interface DeployContextState {
  project: string;
  namespace: string;
  chainedFrom: string;
  stackProgress: StackProgress | null;

  setProject: (project: string) => void;
  setNamespace: (namespace: string) => void;
  startChain: (from: string, total: number) => void;
  advanceChain: () => void;
  endChain: () => void;
  clearContext: () => void;
}

export const useDeployContextStore = create<DeployContextState>()(
  persist(
    (set, get) => ({
      project: "",
      namespace: "",
      chainedFrom: "",
      stackProgress: null,

      setProject: (project) => set({ project }),
      setNamespace: (namespace) => set({ namespace }),

      startChain: (from, total) =>
        set({
          chainedFrom: from,
          stackProgress: { current: 1, total },
        }),

      advanceChain: () => {
        const sp = get().stackProgress;
        if (sp) {
          set({ stackProgress: { current: sp.current + 1, total: sp.total } });
        }
      },

      endChain: () =>
        set({ chainedFrom: "", stackProgress: null }),

      clearContext: () =>
        set({
          project: "",
          namespace: "",
          chainedFrom: "",
          stackProgress: null,
        }),
    }),
    {
      name: "deploy-context",
      storage: createJSONStorage(() => sessionStorage),
    }
  )
);
