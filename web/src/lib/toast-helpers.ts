// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { toast } from "sonner";
import { createElement } from "react";
import { CheckCircle2, XCircle, AlertTriangle, Info } from "@/lib/icons";

export interface ToastAction {
  label: string;
  onClick: () => void;
}

export interface ToastOptions {
  description?: string;
  action?: ToastAction;
}

/**
 * Show a success toast. Auto-dismisses after 4 seconds.
 * Use for: create, update, delete confirmations.
 */
export function showSuccessToast(message: string, options?: ToastOptions): void {
  toast.success(message, {
    duration: 4000,
    icon: createElement(CheckCircle2, { size: 18 }),
    description: options?.description,
    action: options?.action
      ? { label: options.action.label, onClick: options.action.onClick }
      : undefined,
  });
}

/**
 * Show an error toast. Persists until manually dismissed.
 * Use for: API failures, validation errors, network errors.
 * Optionally provide an action (e.g. "Retry" button).
 */
export function showErrorToast(message: string, options?: ToastOptions): void {
  toast.error(message, {
    duration: Infinity,
    icon: createElement(XCircle, { size: 18 }),
    description: options?.description,
    action: options?.action
      ? { label: options.action.label, onClick: options.action.onClick }
      : undefined,
  });
}

/**
 * Show a warning toast. Auto-dismisses after 8 seconds.
 * Use for: policy warnings, rate limits, degraded functionality.
 */
export function showWarningToast(message: string, options?: ToastOptions): void {
  toast.warning(message, {
    duration: 8000,
    icon: createElement(AlertTriangle, { size: 18 }),
    description: options?.description,
    action: options?.action
      ? { label: options.action.label, onClick: options.action.onClick }
      : undefined,
  });
}

/**
 * Show an info toast. Auto-dismisses after 4 seconds.
 * Use for: background sync complete, informational notices.
 */
export function showInfoToast(message: string, options?: ToastOptions): void {
  toast.info(message, {
    duration: 4000,
    icon: createElement(Info, { size: 18 }),
    description: options?.description,
    action: options?.action
      ? { label: options.action.label, onClick: options.action.onClick }
      : undefined,
  });
}
