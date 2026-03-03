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
});

// Request interceptor: Add JWT token to requests
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('jwt_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

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
