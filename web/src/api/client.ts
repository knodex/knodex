// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import axios from "axios";
import { logger } from "@/lib/logger";
import { useUserStore } from "@/stores/userStore";

// Backend API error response structure
export interface ApiErrorResponse {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

// Custom error class that preserves backend error structure
export class ApiError extends Error {
  code: string;
  status: number;
  details?: Record<string, unknown>;

  constructor(code: string, message: string, status: number, details?: Record<string, unknown>) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status;
    this.details = details;
  }
}

// Type guard to check if response matches backend error format
function isApiErrorResponse(data: unknown): data is ApiErrorResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'code' in data &&
    'message' in data &&
    typeof (data as ApiErrorResponse).code === 'string' &&
    typeof (data as ApiErrorResponse).message === 'string'
  );
}

// Auth-related paths that should NOT trigger 401 redirect
const AUTH_PATHS = ['/login', '/auth/callback'];

// Timestamp-based cooldown to prevent multiple simultaneous 401 redirects
let lastRedirectTimestamp = 0;
const REDIRECT_COOLDOWN_MS = 2000;

// Exported for testing
export function _resetRedirectState() {
  lastRedirectTimestamp = 0;
}

export function _getLastRedirectTimestamp() {
  return lastRedirectTimestamp;
}

const apiClient = axios.create({
  baseURL: "/api",
  headers: {
    "Content-Type": "application/json",
  },
  withCredentials: true, // Send HttpOnly session cookie with all requests
});

// Request interceptor: Attach JWT token as Bearer header for E2E/API-key auth.
// The server accepts both HttpOnly cookies and Authorization headers (see middleware/auth.go).
// In production, the HttpOnly cookie is the primary auth mechanism. The Bearer header
// serves as a fallback for E2E tests (which inject tokens into localStorage) and
// API-key clients that don't use cookies.
apiClient.interceptors.request.use((config) => {
  const token = typeof window !== 'undefined' ? localStorage.getItem('jwt_token') : null;
  if (token && !config.headers.Authorization) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

// Response interceptor: Handle errors
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    logger.error("[API] Error:", error.response?.data || error.message);

    // Handle 401 Unauthorized - redirect to login
    if (error.response?.status === 401) {
      const isAuthPath = AUTH_PATHS.some(path => window.location.pathname.startsWith(path));

      const now = Date.now();
      if (!isAuthPath && (now - lastRedirectTimestamp >= REDIRECT_COOLDOWN_MS)) {
        lastRedirectTimestamp = now;
        // Use Zustand store logout (clears both store and localStorage) instead of hard redirect
        useUserStore.getState().logout();
      }
    }

    // Extract structured backend error if available
    const responseData = error.response?.data;
    if (isApiErrorResponse(responseData)) {
      const apiError = new ApiError(
        responseData.code,
        responseData.message,
        error.response.status,
        responseData.details
      );
      logger.debug(`[API] Structured error: code=${apiError.code}, status=${apiError.status}`);
      return Promise.reject(apiError);
    }

    // Fallback for non-standard error responses
    if (error.response?.status) {
      const fallbackError = new ApiError(
        'UNKNOWN_ERROR',
        error.message || 'An unexpected error occurred',
        error.response.status
      );
      return Promise.reject(fallbackError);
    }

    return Promise.reject(error);
  }
);

export default apiClient;
