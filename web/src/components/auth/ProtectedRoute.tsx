// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Navigate, useLocation } from 'react-router-dom';
import { hasPersistedSession, useSessionStatus } from '@/hooks/useAuth';
import { useSessionRestore } from '@/hooks/useSessionRestore';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const location = useLocation();
  const sessionStatus = useSessionStatus();
  useSessionRestore();

  // Phase 1: Sync localStorage check — no session marker means no prior login
  if (!hasPersistedSession()) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Phase 2: Check session status from store
  if (sessionStatus === 'logged_out') {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // All other states (idle, validating, valid, error) — render children
  // DashboardLayout handles content-area spinner/error
  return <>{children}</>;
}
