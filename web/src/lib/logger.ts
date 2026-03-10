// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

/**
 * Lightweight logger utility for environment-aware logging.
 *
 * - debug/info: Only in development
 * - warn/error: Always logged (useful for production debugging)
 *
 * Usage:
 *   import { logger } from '@/lib/logger';
 *   logger.debug('[Component]', 'message', data);
 *   logger.error('[Component]', 'Failed to fetch:', error);
 */

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

interface Logger {
  debug: (...args: unknown[]) => void;
  info: (...args: unknown[]) => void;
  warn: (...args: unknown[]) => void;
  error: (...args: unknown[]) => void;
  log: (...args: unknown[]) => void; // Alias for debug
}

const isDev = import.meta.env.DEV;

/**
 * Create a prefixed logger for a specific module/component.
 * Useful for consistent prefixing without repeating the tag.
 *
 * Usage:
 *   const log = createLogger('[WebSocket]');
 *   log.debug('Connected');
 *   log.error('Failed:', error);
 */
export function createLogger(prefix: string): Logger {
  return {
    debug: (...args: unknown[]) => logger.debug(prefix, ...args),
    info: (...args: unknown[]) => logger.info(prefix, ...args),
    warn: (...args: unknown[]) => logger.warn(prefix, ...args),
    error: (...args: unknown[]) => logger.error(prefix, ...args),
    log: (...args: unknown[]) => logger.debug(prefix, ...args),
  };
}

export const logger: Logger = {
  /**
   * Debug level - development only.
   * Use for verbose debugging, state dumps, tracing.
   */
  debug: (...args: unknown[]) => {
    if (isDev) {
      console.log(...args);
    }
  },

  /**
   * Info level - development only.
   * Use for general information, successful operations.
   */
  info: (...args: unknown[]) => {
    if (isDev) {
      console.info(...args);
    }
  },

  /**
   * Warn level - always logged.
   * Use for recoverable issues, deprecations, unexpected states.
   */
  warn: (...args: unknown[]) => {
    console.warn(...args);
  },

  /**
   * Error level - always logged.
   * Use for failures, exceptions, unrecoverable states.
   */
  error: (...args: unknown[]) => {
    console.error(...args);
  },

  /**
   * Alias for debug (matches console.log usage pattern).
   */
  log: (...args: unknown[]) => {
    if (isDev) {
      console.log(...args);
    }
  },
};

export default logger;
