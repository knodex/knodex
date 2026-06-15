// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import {
  dateInputToExpiresAt,
  expiresAtToDateInput,
  statusLabel,
  statusBadgeClasses,
  expiryHint,
  metadataEqual,
  emptyMetadataValue,
} from "./secret-metadata";

describe("secret-metadata", () => {
  describe("dateInputToExpiresAt", () => {
    it("returns undefined for empty input", () => {
      expect(dateInputToExpiresAt("")).toBeUndefined();
    });

    it("appends end-of-day UTC for a date", () => {
      expect(dateInputToExpiresAt("2026-01-15")).toBe("2026-01-15T23:59:59Z");
    });
  });

  describe("expiresAtToDateInput", () => {
    it("returns empty string for undefined", () => {
      expect(expiresAtToDateInput(undefined)).toBe("");
    });

    it("returns empty string for malformed input", () => {
      expect(expiresAtToDateInput("not-a-date")).toBe("");
    });

    it("slices a valid RFC3339 timestamp to YYYY-MM-DD", () => {
      expect(expiresAtToDateInput("2026-01-15T23:59:59Z")).toBe("2026-01-15");
    });
  });

  describe("statusLabel", () => {
    it.each([
      ["active", "Active"],
      ["expiring-soon", "Expiring soon"],
      ["expired", "Expired"],
    ] as const)("maps %s to %s", (status, label) => {
      expect(statusLabel(status)).toBe(label);
    });

    it("renders an em-dash for undefined", () => {
      expect(statusLabel(undefined)).toBe("—");
    });
  });

  describe("statusBadgeClasses", () => {
    it("returns distinct classes per status", () => {
      expect(statusBadgeClasses("active")).toContain("emerald");
      expect(statusBadgeClasses("expiring-soon")).toContain("amber");
      expect(statusBadgeClasses("expired")).toContain("rose");
      expect(statusBadgeClasses(undefined)).toContain("muted");
    });
  });

  describe("expiryHint", () => {
    const now = new Date("2026-01-15T00:00:00Z");

    it("returns empty string when no expiration", () => {
      expect(expiryHint(undefined, now)).toBe("");
    });

    it("returns empty string for malformed input", () => {
      expect(expiryHint("nope", now)).toBe("");
    });

    it("reports past expiry", () => {
      expect(expiryHint("2026-01-13T00:00:00Z", now)).toBe("Expired 2d ago");
    });

    it("reports today", () => {
      expect(expiryHint("2026-01-15T00:00:00Z", now)).toBe("Expires today");
    });

    it("reports tomorrow", () => {
      expect(expiryHint("2026-01-16T00:00:00Z", now)).toBe("Expires tomorrow");
    });

    it("reports days remaining", () => {
      expect(expiryHint("2026-01-20T00:00:00Z", now)).toBe("Expires in 5d");
    });

    it("uses the default 'now' when not supplied", () => {
      // A far-future date is always "Expires in Nd" regardless of the real clock.
      expect(expiryHint("2999-01-01T00:00:00Z")).toMatch(/^Expires in \d+d$/);
    });
  });

  describe("metadataEqual", () => {
    it("treats two undefined snapshots as equal", () => {
      expect(metadataEqual(undefined, undefined)).toBe(true);
    });

    it("normalizes missing fields to empty when comparing", () => {
      expect(
        metadataEqual({ rotation: "90d" } as never, {
          rotation: "90d",
          docsUrl: "",
          expiresAt: "",
        } as never),
      ).toBe(true);
    });

    it("detects a difference in any field", () => {
      expect(
        metadataEqual(
          { docsUrl: "a" } as never,
          { docsUrl: "b" } as never,
        ),
      ).toBe(false);
    });

    it("treats a populated snapshot as unequal to undefined", () => {
      expect(
        metadataEqual({ rotation: "90d", docsUrl: "u", expiresAt: "x" } as never, undefined),
      ).toBe(false);
      expect(
        metadataEqual(undefined, { rotation: "90d" } as never),
      ).toBe(false);
    });
  });

  it("exposes a blank default form value", () => {
    expect(emptyMetadataValue).toEqual({
      rotation: "",
      docsUrl: "",
      expiresAtDate: "",
    });
  });
});
