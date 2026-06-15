// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { formatDistanceToNow, formatDate, formatDateTime } from "./date";

describe("date utilities", () => {
  describe("formatDistanceToNow", () => {
    const fixedNow = new Date("2026-01-15T12:00:00Z");

    beforeEach(() => {
      vi.useFakeTimers();
      vi.setSystemTime(fixedNow);
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("renders 'just now' for sub-minute deltas", () => {
      expect(formatDistanceToNow("2026-01-15T11:59:30Z")).toBe("just now");
    });

    it("renders minutes", () => {
      expect(formatDistanceToNow("2026-01-15T11:45:00Z")).toBe("15m ago");
    });

    it("renders hours", () => {
      expect(formatDistanceToNow("2026-01-15T09:00:00Z")).toBe("3h ago");
    });

    it("renders days", () => {
      expect(formatDistanceToNow("2026-01-10T12:00:00Z")).toBe("5d ago");
    });

    it("falls back to a locale date for deltas over 30 days", () => {
      const result = formatDistanceToNow("2025-11-01T12:00:00Z");
      expect(result).not.toMatch(/ago|just now/);
      expect(result.length).toBeGreaterThan(0);
    });
  });

  describe("formatDate", () => {
    it("returns a non-empty formatted date", () => {
      expect(formatDate("2026-01-15T12:00:00Z")).toMatch(/2026/);
    });
  });

  describe("formatDateTime", () => {
    it("returns a non-empty formatted date-time", () => {
      const result = formatDateTime("2026-01-15T12:00:00Z");
      expect(result).toMatch(/2026/);
      expect(result.length).toBeGreaterThan(0);
    });
  });
});
