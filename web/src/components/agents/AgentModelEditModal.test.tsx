// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { vi } from "vitest";
import { AgentModelEditModal, type EditableAgent } from "./AgentModelEditModal";
import * as agentsApi from "@/api/agents";
import { ApiError } from "@/api/client";

vi.mock("@/api/agents");
vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));
import { toast } from "sonner";

const configs = [
  { name: "azure-cfg", provider: "azure", model: "gpt-4" },
  { name: "openai-cfg", provider: "openai", model: "gpt-4o" },
];

function renderModal(agent: EditableAgent, onOpenChange = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <AgentModelEditModal open onOpenChange={onOpenChange} agent={agent} />
    </QueryClientProvider>
  );
  return { ...utils, onOpenChange };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(agentsApi.getModelConfigs).mockResolvedValue({ modelConfigs: configs });
});

describe("AgentModelEditModal", () => {
  it("lists the namespace's ModelConfigs and pre-selects the bound config by name", async () => {
    renderModal({
      name: "byoa",
      namespace: "alpha-apps",
      modelConfig: "openai-cfg",
    });

    // Fetched for the agent's namespace + name.
    await waitFor(() =>
      expect(agentsApi.getModelConfigs).toHaveBeenCalledWith("alpha-apps", "byoa")
    );
    // The trigger reflects the pre-selected bound config (openai-cfg).
    await waitFor(() =>
      expect(screen.getByTestId("model-config-select")).toHaveTextContent("openai-cfg")
    );
  });

  it("pre-selects by EXACT name, not by provider/model — no silent swap on a collision", async () => {
    // Two configs share {openai, gpt-4o}; the agent is bound to the SECOND.
    vi.mocked(agentsApi.getModelConfigs).mockResolvedValue({
      modelConfigs: [
        { name: "gpt4o-dev", provider: "openai", model: "gpt-4o" },
        { name: "gpt4o-prod", provider: "openai", model: "gpt-4o" },
      ],
    });
    renderModal({
      name: "byoa",
      namespace: "alpha-apps",
      model: { provider: "openai", name: "gpt-4o" },
      modelConfig: "gpt4o-prod",
    });

    // Must show the bound 'gpt4o-prod', NOT the first-sorted 'gpt4o-dev'.
    await waitFor(() =>
      expect(screen.getByTestId("model-config-select")).toHaveTextContent("gpt4o-prod")
    );
    expect(screen.getByTestId("model-config-select")).not.toHaveTextContent("gpt4o-dev");
  });

  it("saves the selected ModelConfig, toasts, and closes", async () => {
    vi.mocked(agentsApi.patchAgentModel).mockResolvedValue({ provider: "openai", name: "gpt-4o" });
    const { onOpenChange } = renderModal({
      name: "byoa",
      namespace: "alpha-apps",
      modelConfig: "openai-cfg",
    });

    await waitFor(() =>
      expect(screen.getByTestId("model-config-select")).toHaveTextContent("openai-cfg")
    );
    await userEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() =>
      expect(agentsApi.patchAgentModel).toHaveBeenCalledWith("alpha-apps", "byoa", "openai-cfg")
    );
    expect(toast.success).toHaveBeenCalled();
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("surfaces a clear error when the ModelConfig 404s and stays open", async () => {
    vi.mocked(agentsApi.patchAgentModel).mockRejectedValue(
      new ApiError("NOT_FOUND", "modelconfig not found", 404)
    );
    const { onOpenChange } = renderModal({
      name: "byoa",
      namespace: "alpha-apps",
      modelConfig: "azure-cfg",
    });

    await waitFor(() =>
      expect(screen.getByTestId("model-config-select")).toHaveTextContent("azure-cfg")
    );
    await userEvent.click(screen.getByRole("button", { name: /save/i }));

    await waitFor(() => expect(toast.error).toHaveBeenCalled());
    // Did not close on failure.
    expect(onOpenChange).not.toHaveBeenCalledWith(false);
  });

  it("disables Save when there are no ModelConfigs to choose", async () => {
    vi.mocked(agentsApi.getModelConfigs).mockResolvedValue({ modelConfigs: [] });
    renderModal({ name: "byoa", namespace: "empty-ns", model: null });

    await waitFor(() =>
      expect(screen.getByText(/No model configurations in this namespace/i)).toBeInTheDocument()
    );
    expect(screen.getByRole("button", { name: /save/i })).toBeDisabled();
  });
});
