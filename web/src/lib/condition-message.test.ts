// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { formatConditionMessage } from "./condition-message";

describe("formatConditionMessage", () => {
  it("returns empty string unchanged", () => {
    expect(formatConditionMessage("")).toBe("");
  });

  it("returns plain messages unchanged", () => {
    const msg = "All resources are ready";
    expect(formatConditionMessage(msg)).toBe(msg);
  });

  it("formats Gatekeeper admission denial", () => {
    const raw =
      'admission webhook "validation.gatekeeper.sh" denied the request: [require-team-label-deployments] Missing required labels: {"team"}';
    expect(formatConditionMessage(raw)).toBe(
      'Blocked by Gatekeeper policy "require-team-label-deployments": Missing required labels: {"team"}'
    );
  });

  it("formats generic admission webhook denial", () => {
    const raw =
      'admission webhook "my-custom-webhook.example.com" denied the request: value must be positive';
    expect(formatConditionMessage(raw)).toBe(
      'Blocked by admission webhook "my-custom-webhook.example.com": value must be positive'
    );
  });

  it("strips KRO reconciliation prefix before parsing", () => {
    const raw =
      'resource reconciliation failed: apply results contain errors: admission webhook "validation.gatekeeper.sh" denied the request: [require-owner-annotation-simpleapp] Missing required annotations: {"owner"}';
    expect(formatConditionMessage(raw)).toBe(
      'Blocked by Gatekeeper policy "require-owner-annotation-simpleapp": Missing required annotations: {"owner"}'
    );
  });

  it("strips KRO prefix from generic webhook denial", () => {
    const raw =
      'resource reconciliation failed: apply results contain errors: admission webhook "opa.example.com" denied the request: policy violation';
    expect(formatConditionMessage(raw)).toBe(
      'Blocked by admission webhook "opa.example.com": policy violation'
    );
  });

  it("strips KRO prefix from plain error message", () => {
    const raw =
      "resource reconciliation failed: apply results contain errors: timeout waiting for resource";
    expect(formatConditionMessage(raw)).toBe("timeout waiting for resource");
  });

  it("trims trailing whitespace from Gatekeeper reason", () => {
    const raw =
      'admission webhook "validation.gatekeeper.sh" denied the request: [my-constraint] reason with trailing spaces   ';
    const result = formatConditionMessage(raw);
    expect(result).toBe(
      'Blocked by Gatekeeper policy "my-constraint": reason with trailing spaces'
    );
  });

  it("trims trailing whitespace from generic webhook reason", () => {
    const raw =
      'admission webhook "wh.example.com" denied the request: reason   ';
    const result = formatConditionMessage(raw);
    expect(result).toBe('Blocked by admission webhook "wh.example.com": reason');
  });

  it("does not match partial webhook denial strings", () => {
    const msg = "the webhook processed the request: all good";
    expect(formatConditionMessage(msg)).toBe(msg);
  });
});
