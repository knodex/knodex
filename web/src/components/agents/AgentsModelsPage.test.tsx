// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AgentsModelsPage } from "./AgentsModelsPage";
import * as agentsApi from "@/api/agents";
import type { ModelSummary } from "@/api/agents";

vi.mock("@/api/agents");
// The Create Model dialog pulls project/namespace hooks; stub them so the page
// test stays focused on the list + the dialog opening.
vi.mock("@/hooks/useProjects", () => ({
  useProjects: () => ({ data: { items: [{ name: "alpha" }] }, isLoading: false }),
}));
vi.mock("@/hooks/useNamespaces", () => ({
  useProjectNamespaces: () => ({ data: { namespaces: ["alpha-apps"] }, isLoading: false, isError: false }),
}));
vi.mock("@/hooks/useAuth", () => ({ useCurrentProject: () => "" }));

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AgentsModelsPage />
    </QueryClientProvider>
  );
}

const models: ModelSummary[] = [
  { name: "alpha-model", namespace: "alpha-apps", provider: "openai", model: "gpt-4o" },
  { name: "beta-model", namespace: "beta-apps", provider: "anthropic", model: "claude" },
];

describe("AgentsModelsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders one row per model with name, namespace, provider and model", async () => {
    vi.mocked(agentsApi.listModels).mockResolvedValue({ models });

    renderPage();

    expect(await screen.findByText("alpha-model")).toBeInTheDocument();
    expect(screen.getByText("beta-model")).toBeInTheDocument();
    expect(screen.getByText("openai")).toBeInTheDocument();
    expect(screen.getByText("gpt-4o")).toBeInTheDocument();
    expect(screen.getAllByTestId("agents-models-row")).toHaveLength(2);
  });

  it("shows an empty state that explains a model is needed before deploying an agent", async () => {
    vi.mocked(agentsApi.listModels).mockResolvedValue({ models: [] });

    renderPage();

    expect(await screen.findByText("No models yet")).toBeInTheDocument();
    expect(screen.getByText(/before you can deploy an agent/i)).toBeInTheDocument();
  });

  it("opens the Create Model dialog from the header button", async () => {
    vi.mocked(agentsApi.listModels).mockResolvedValue({ models });

    renderPage();

    await screen.findByText("alpha-model");
    await userEvent.click(screen.getByTestId("create-model-button"));

    // Dialog title surfaces once open.
    expect(await screen.findByRole("heading", { name: "Create Model" })).toBeInTheDocument();
  });

  it("shows a retryable error when the fetch fails", async () => {
    vi.mocked(agentsApi.listModels).mockRejectedValueOnce(new Error("network down"));

    renderPage();

    expect(await screen.findByTestId("agents-models-error")).toBeInTheDocument();
  });
});
