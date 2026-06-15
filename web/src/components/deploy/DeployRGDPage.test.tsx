// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { DeployRGDPage } from "./DeployRGDPage";
import * as rgdApi from "@/api/rgd";
import { ApiError } from "@/api/client";
import { useCanI } from "@/hooks/useCanI";

vi.mock("@/api/rgd");
vi.mock("@/hooks/useCanI");

const RGD_YAML = `apiVersion: kro.run/v1alpha1
kind: ResourceGraphDefinition
metadata:
  name: webapp-stack
  namespace: platform
spec:
  schema:
    apiVersion: v1alpha1
    kind: WebAppStack
  resources:
    - id: deployment
      template:
        apiVersion: apps/v1
        kind: Deployment`;

function renderPage(state?: { specYaml?: string; requirement?: string; runId?: string }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter
        initialEntries={[{ pathname: "/deploy-rgd", state: state ?? null }]}
      >
        <Routes>
          <Route path="/deploy-rgd" element={<DeployRGDPage />} />
          <Route
            path="/agents/list"
            element={<div data-testid="builder-page">builder</div>}
          />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>
  );
}

describe("DeployRGDPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCanI).mockReturnValue({ allowed: true, isLoading: false, isError: false });
    vi.mocked(rgdApi.createRGD).mockResolvedValue({
      name: "webapp-stack",
      namespace: "platform",
      kind: "ResourceGraphDefinition",
      apiVersion: "kro.run/v1alpha1",
    });
  });

  it("redirects to the agents list when opened without router state", () => {
    renderPage(undefined);

    expect(screen.getByTestId("builder-page")).toBeInTheDocument();
    expect(screen.queryByTestId("deploy-rgd-form")).not.toBeInTheDocument();
  });

  it("prefills name and YAML from the generated spec", () => {
    renderPage({ specYaml: RGD_YAML, runId: "run-1" });

    expect(screen.getByTestId("deploy-rgd-name")).toHaveValue("webapp-stack");
    expect(screen.queryByTestId("deploy-rgd-namespace")).not.toBeInTheDocument();
    expect(screen.getByTestId("deploy-rgd-yaml")).toHaveValue(RGD_YAML);
  });

  it("keeps the YAML fully editable (AC #4)", async () => {
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    const textarea = screen.getByTestId("deploy-rgd-yaml") as HTMLTextAreaElement;
    await user.click(textarea);
    await user.keyboard("# user edit{Enter}");
    expect(textarea.value).toContain("# user edit");
  });

  it("keeps name editable independently of the YAML", async () => {
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    const nameInput = screen.getByTestId("deploy-rgd-name");
    await user.clear(nameInput);
    await user.type(nameInput, "renamed-stack");
    expect(nameInput).toHaveValue("renamed-stack");

    // The YAML stayed untouched by the field edit.
    expect(screen.getByTestId("deploy-rgd-yaml")).toHaveValue(RGD_YAML);
  });

  it("shows an inline error WITHOUT posting when the edited YAML is not an RGD", async () => {
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    const textarea = screen.getByTestId("deploy-rgd-yaml");
    await user.clear(textarea);
    await user.click(textarea);
    await user.keyboard("kind: Deployment");

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    expect(await screen.findByTestId("deploy-rgd-error")).toHaveTextContent(
      "ResourceGraphDefinition"
    );
    expect(rgdApi.createRGD).not.toHaveBeenCalled();
  });

  it("submits the CURRENT (edited) textarea content with name and runId", async () => {
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML, runId: "run-42" });

    // Edit the YAML — the edit must reach the server (AC #4).
    const textarea = screen.getByTestId("deploy-rgd-yaml");
    await user.click(textarea);
    await user.keyboard("{Control>}{End}{/Control}{Enter}# tweaked");

    const nameInput = screen.getByTestId("deploy-rgd-name");
    await user.clear(nameInput);
    await user.type(nameInput, "renamed-stack");

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    await waitFor(() => expect(rgdApi.createRGD).toHaveBeenCalledTimes(1));
    const request = vi.mocked(rgdApi.createRGD).mock.calls[0][0];
    expect(request.name).toBe("renamed-stack");
    expect(request.namespace).toBeUndefined();
    expect(request.runId).toBe("run-42");
    expect(request.yaml).toContain("# tweaked");
  });

  it("renders the success panel with Catalog link after a 201", async () => {
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML, runId: "run-1" });

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    const panel = await screen.findByTestId("deploy-rgd-success");
    expect(panel).toHaveTextContent("ResourceGraphDefinition webapp-stack created");
    expect(panel).toHaveTextContent(/Catalog/);
    expect(panel).toHaveTextContent(/KRO reconciles/);
    expect(screen.getByTestId("deploy-rgd-view-catalog")).toHaveAttribute(
      "href",
      "/catalog/webapp-stack"
    );
    expect(screen.getByTestId("deploy-rgd-back-builder")).toHaveAttribute(
      "href",
      "/agents/list"
    );
  });

  it("renders a clear authorization message on 403", async () => {
    vi.mocked(rgdApi.createRGD).mockRejectedValue(
      new ApiError("FORBIDDEN", "permission denied", 403)
    );
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    expect(await screen.findByTestId("deploy-rgd-error")).toHaveTextContent(
      "not authorized"
    );
  });

  it("renders a rename suggestion on 409 name conflict", async () => {
    vi.mocked(rgdApi.createRGD).mockRejectedValue(
      new ApiError("CONFLICT", "ResourceGraphDefinition 'webapp-stack' already exists", 409)
    );
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    const error = await screen.findByTestId("deploy-rgd-error");
    expect(error).toHaveTextContent("already exists");
    expect(error).toHaveTextContent("rename");
  });

  it("surfaces other ApiError messages verbatim", async () => {
    vi.mocked(rgdApi.createRGD).mockRejectedValue(
      new ApiError("BAD_REQUEST", "namespace not found", 400)
    );
    const user = userEvent.setup();
    renderPage({ specYaml: RGD_YAML });

    await user.click(screen.getByTestId("deploy-rgd-submit"));

    expect(await screen.findByTestId("deploy-rgd-error")).toHaveTextContent(
      "namespace not found"
    );
  });

  it("shows the best-effort can-i notice when denied (server stays authoritative)", () => {
    vi.mocked(useCanI).mockReturnValue({ allowed: false, isLoading: false, isError: false });
    renderPage({ specYaml: RGD_YAML });

    expect(screen.getByTestId("deploy-rgd-cani-notice")).toBeInTheDocument();
  });
});
