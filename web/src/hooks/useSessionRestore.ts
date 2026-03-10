// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect, useRef, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useUserStore } from '@/stores/userStore';
import { getAccountInfo } from '@/api/auth';
import { ApiError } from '@/api/client';

/**
 * Hook that validates the user's session against the server on mount
 * and when the session status transitions back to 'idle' (e.g., retry after error).
 *
 * Uses Zustand's subscribe() to watch for idle transitions, ensuring the retry
 * button in DashboardLayout actually re-triggers the restore flow.
 */
export function useSessionRestore(): void {
  const queryClient = useQueryClient();
  const controllerRef = useRef<AbortController | null>(null);
  const mountedRef = useRef(true);

  const startRestore = useCallback(() => {
    const { sessionStatus } = useUserStore.getState();
    if (sessionStatus !== 'idle' || !mountedRef.current) return;

    // Abort any previous in-flight request (e.g., from StrictMode double-mount)
    controllerRef.current?.abort();

    const controller = new AbortController();
    controllerRef.current = controller;

    useUserStore.getState().setSessionStatus('validating');

    getAccountInfo({ signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        useUserStore.getState().restoreSession(data);
        queryClient.invalidateQueries();
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        if (error instanceof Error && error.name === 'CanceledError') return;
        const status = error instanceof ApiError
          ? error.status
          : (typeof error === 'object' && error !== null && 'response' in error)
            ? (error as { response: { status?: number } }).response?.status
            : undefined;
        if (status === 401 || status === 403) {
          useUserStore.getState().logout();
        } else {
          useUserStore.getState().setSessionStatus(
            'error',
            'Unable to connect to server. Please check your connection.'
          );
        }
      });
  }, [queryClient]);

  useEffect(() => {
    mountedRef.current = true;

    // Initial restore attempt on mount
    startRestore();

    // Subscribe to store — retry when status transitions to 'idle' (e.g., retry button)
    const unsubscribe = useUserStore.subscribe(
      (state, prevState) => {
        if (state.sessionStatus === 'idle' && prevState.sessionStatus !== 'idle') {
          startRestore();
        }
      }
    );

    return () => {
      mountedRef.current = false;
      unsubscribe();
      controllerRef.current?.abort();
      // Reset to idle if we were mid-validation (for StrictMode remount)
      if (useUserStore.getState().sessionStatus === 'validating') {
        useUserStore.getState().setSessionStatus('idle');
      }
    };
  }, [startRestore]);
}
