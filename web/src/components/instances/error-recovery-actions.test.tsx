// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { NeedsAttentionBanner, humanizeError } from "./error-recovery-actions";

describe("NeedsAttentionBanner", () => {
  it("renders nothing when failedCount is 0", () => {
    const { container } = render(<NeedsAttentionBanner failedCount={0} />);
    expect(container.firstChild).toBeNull();
  });

  it("renders banner with correct count", () => {
    render(<NeedsAttentionBanner failedCount={2} />);
    expect(screen.getByText("2 resources need attention")).toBeInTheDocument();
    expect(screen.getByRole("button")).toBeInTheDocument();
  });

  it("uses singular form for 1 resource", () => {
    render(<NeedsAttentionBanner failedCount={1} />);
    expect(screen.getByText("1 resource needs attention")).toBeInTheDocument();
  });
});

describe("humanizeError", () => {
  it("maps CrashLoopBackOff to friendly message", () => {
    const msg = humanizeError("Back-off restarting failed container: CrashLoopBackOff");
    expect(msg).toContain("keeps crashing");
  });

  it("maps ImagePullBackOff to friendly message", () => {
    const msg = humanizeError("ImagePullBackOff: unable to pull image");
    expect(msg).toContain("pull the container image");
  });

  it("returns fallback for unknown errors", () => {
    const msg = humanizeError("some unknown k8s error xyz");
    expect(msg).toContain("Something went wrong");
  });

  it("does not expose raw K8s jargon in fallback", () => {
    const msg = humanizeError("random error");
    expect(msg).not.toContain("random error");
  });
});
