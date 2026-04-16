// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { createElement } from "react";
import { CheckCircle2, XCircle, AlertTriangle, Info } from "@/lib/icons";
import {
  showSuccessToast,
  showErrorToast,
  showWarningToast,
  showInfoToast,
} from "./toast-helpers";

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    warning: vi.fn(),
    info: vi.fn(),
  },
}));

// Import after mock so we get the mocked version
import { toast } from "sonner";

beforeEach(() => {
  vi.clearAllMocks();
});

describe("showSuccessToast", () => {
  it("calls toast.success with correct icon and 4000ms duration", () => {
    showSuccessToast("Item created");

    expect(toast.success).toHaveBeenCalledOnce();
    expect(toast.success).toHaveBeenCalledWith("Item created", {
      duration: 4000,
      icon: createElement(CheckCircle2, { size: 18 }),
      description: undefined,
      action: undefined,
    });
  });

  it("forwards description when provided", () => {
    showSuccessToast("Saved", { description: "All changes saved." });

    expect(toast.success).toHaveBeenCalledWith("Saved", expect.objectContaining({
      description: "All changes saved.",
    }));
  });

  it("forwards action when provided", () => {
    const onClick = vi.fn();
    showSuccessToast("Done", { action: { label: "View", onClick } });

    expect(toast.success).toHaveBeenCalledWith("Done", expect.objectContaining({
      action: { label: "View", onClick },
    }));
  });
});

describe("showErrorToast", () => {
  it("calls toast.error with correct icon and Infinity duration", () => {
    showErrorToast("Something failed");

    expect(toast.error).toHaveBeenCalledOnce();
    expect(toast.error).toHaveBeenCalledWith("Something failed", {
      duration: Infinity,
      icon: createElement(XCircle, { size: 18 }),
      description: undefined,
      action: undefined,
    });
  });

  it("forwards description when provided", () => {
    showErrorToast("Deploy failed", { description: "Timeout after 30s" });

    expect(toast.error).toHaveBeenCalledWith("Deploy failed", expect.objectContaining({
      description: "Timeout after 30s",
    }));
  });

  it("forwards action (e.g. Retry) when provided", () => {
    const onClick = vi.fn();
    showErrorToast("Request failed", { action: { label: "Retry", onClick } });

    expect(toast.error).toHaveBeenCalledWith("Request failed", expect.objectContaining({
      action: { label: "Retry", onClick },
    }));
  });
});

describe("showWarningToast", () => {
  it("calls toast.warning with correct icon and 8000ms duration", () => {
    showWarningToast("Policy warning detected");

    expect(toast.warning).toHaveBeenCalledOnce();
    expect(toast.warning).toHaveBeenCalledWith("Policy warning detected", {
      duration: 8000,
      icon: createElement(AlertTriangle, { size: 18 }),
      description: undefined,
      action: undefined,
    });
  });

  it("forwards description when provided", () => {
    showWarningToast("Rate limited", { description: "Too many requests" });

    expect(toast.warning).toHaveBeenCalledWith("Rate limited", expect.objectContaining({
      description: "Too many requests",
    }));
  });

  it("forwards action when provided", () => {
    const onClick = vi.fn();
    showWarningToast("Review required", { action: { label: "View details", onClick } });

    expect(toast.warning).toHaveBeenCalledWith("Review required", expect.objectContaining({
      action: { label: "View details", onClick },
    }));
  });
});

describe("showInfoToast", () => {
  it("calls toast.info with correct icon and 4000ms duration", () => {
    showInfoToast("Sync complete");

    expect(toast.info).toHaveBeenCalledOnce();
    expect(toast.info).toHaveBeenCalledWith("Sync complete", {
      duration: 4000,
      icon: createElement(Info, { size: 18 }),
      description: undefined,
      action: undefined,
    });
  });

  it("forwards description when provided", () => {
    showInfoToast("Sync complete", { description: "12 resources updated" });

    expect(toast.info).toHaveBeenCalledWith("Sync complete", expect.objectContaining({
      description: "12 resources updated",
    }));
  });

  it("forwards action when provided", () => {
    const onClick = vi.fn();
    showInfoToast("Update available", { action: { label: "View changelog", onClick } });

    expect(toast.info).toHaveBeenCalledWith("Update available", expect.objectContaining({
      action: { label: "View changelog", onClick },
    }));
  });
});
