// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { DeployTimeline } from "./deploy-timeline";

beforeAll(() => {
  Element.prototype.scrollIntoView = vi.fn();
});

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

function renderTimeline() {
  return render(
    <MemoryRouter>
      <DeployTimeline
        instanceId="inst-123"
        instanceName="my-postgres"
        namespace="default"
        kind="MyDB"
        rgdName="postgres-rgd"
      />
    </MemoryRouter>
  );
}

describe("DeployTimeline", () => {
  it("renders progress bar", () => {
    const { container } = renderTimeline();
    const bar = container.querySelector("[data-testid='deploy-timeline']");
    expect(bar).toBeInTheDocument();
  });

  it("shows timeline entries after creation events", async () => {
    renderTimeline();
    await waitFor(() => {
      expect(screen.getByText(/my-postgres/)).toBeInTheDocument();
    }, { timeout: 1000 });
  });

  it("shows success state with View Instance button", async () => {
    renderTimeline();
    await waitFor(() => {
      expect(screen.getByText("Deployment successful")).toBeInTheDocument();
      expect(screen.getByText("View Instance")).toBeInTheDocument();
    }, { timeout: 3000 });
  });

  it("has data-testid for integration tests", () => {
    renderTimeline();
    expect(screen.getByTestId("deploy-timeline")).toBeInTheDocument();
  });
});
