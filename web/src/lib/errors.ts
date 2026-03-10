// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Error handling utilities
 * Sanitizes error messages to prevent information disclosure
 */
import { logger } from '@/lib/logger';

/**
 * Patterns that indicate a technical/unsafe error message
 * These should be sanitized to prevent information disclosure
 */
const TECHNICAL_ERROR_PATTERNS = [
  /at\s+[\w.]+\s+\(/i, // Stack trace patterns like "at Function.call ("
  /\/[\w/.-]+\.\w+:\d+/i, // File paths with line numbers like "/path/file.ts:42"
  /\b(TypeError|ReferenceError|SyntaxError|EvalError|RangeError|URIError)\b/i, // JS error types
  /\b(ENOTFOUND|ETIMEDOUT|ECONNRESET|EPIPE|EHOSTUNREACH)\b/i, // System error codes
  /\b(null|undefined)\s+(is not|cannot)/i, // Null/undefined errors
  /\bcannot read propert/i, // Property access errors
  /\bunexpected token/i, // Parse errors
  /\binternal server error\b/i, // Generic server errors that might leak info
  /\b(sql|query|database)\s+(error|exception|failed)/i, // Database errors
  /password|secret|key|token|credential/i, // Potential credential leaks
];

/**
 * Check if an error message contains technical/unsafe content
 */
function containsTechnicalContent(message: string): boolean {
  return TECHNICAL_ERROR_PATTERNS.some(pattern => pattern.test(message));
}

/**
 * Sanitize error messages for user display
 * Prevents information disclosure while maintaining UX
 *
 * Business-level errors (validation, not found, etc.) are passed through.
 * Technical errors (stack traces, internal paths) are sanitized.
 */
export function getSafeErrorMessage(error: unknown): string {
  if (!(error instanceof Error)) {
    return "An unexpected error occurred. Please try again.";
  }

  // Map known error types to user-friendly messages
  const errorMap: Record<string, string> = {
    'Network Error': 'Unable to connect to the server. Please check your connection.',
    'timeout': 'The request took too long. Please try again.',
    'ECONNREFUSED': 'Unable to connect to the server. Please check your connection.',
    '401': 'You need to log in to access this resource.',
    '403': 'You don\'t have permission to access this resource.',
    '404': 'The requested resource was not found.',
    '500': 'A server error occurred. Please try again later.',
    '502': 'The server is temporarily unavailable. Please try again later.',
    '503': 'The service is temporarily unavailable. Please try again later.',
  };

  // Check for known error patterns that need mapping
  for (const [pattern, message] of Object.entries(errorMap)) {
    if (error.message.includes(pattern)) {
      return message;
    }
  }

  // Log the full error for debugging (only in development via logger)
  logger.error('[Error Details]', error);

  // Check if the message contains technical/unsafe content
  if (containsTechnicalContent(error.message)) {
    return "An error occurred. Please try again.";
  }

  // Allow clean business-level error messages through
  // (e.g., "Project name already exists", "User not found")
  if (error.message && error.message.length > 0 && error.message.length < 200) {
    return error.message;
  }

  // Return generic message for anything else
  return "An error occurred. Please try again.";
}

/**
 * Maps API validation error messages to user-friendly equivalents
 * Used across project settings, destinations, and other project management UIs
 */
const PROJECT_ERROR_MAPPINGS: Record<string, string> = {
  // Destination errors
  "at least one destination is required": "Please add at least one deployment destination",
  "destination is required": "Please add at least one deployment destination",
  "namespace is required": "Please specify a namespace for each destination",
  "must specify at least one destination": "Please add at least one deployment destination",
  // Name errors
  "project name is required": "Please enter a project name",
  "name is required": "Please enter a project name",
  "invalid project name": "Project name must be lowercase, alphanumeric, and may contain hyphens",
  // Validation errors
  "invalid project spec": "The project configuration is invalid. Please check all fields.",
  "validation failed": "Please correct the errors in the form and try again.",
  // Duplicate errors
  "already exists": "A project with this name already exists. Please choose a different name",
};

export function toUserFriendlyError(message: string): string {
  const lowerMessage = message.toLowerCase();
  for (const [pattern, friendly] of Object.entries(PROJECT_ERROR_MAPPINGS)) {
    if (lowerMessage.includes(pattern.toLowerCase())) {
      return friendly;
    }
  }
  return message;
}
