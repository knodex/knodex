// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { Navigate, useLocation } from 'react-router-dom';
import { useIsAuthenticated } from '@/hooks/useAuth';
import { useUserStore } from '@/stores/userStore';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const isAuthenticated = useIsAuthenticated();
  const isTokenExpired = useUserStore((state) => state.isTokenExpired);
  const logout = useUserStore((state) => state.logout);
  const location = useLocation();

  // Check if user is authenticated
  if (!isAuthenticated) {
    // Redirect to login page, saving the current location
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // Check if token is expired
  if (isTokenExpired()) {
    // Token expired, log out and redirect
    logout();
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  // User is authenticated and token is valid
  return <>{children}</>;
}
