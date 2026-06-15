// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import { AgentsTemplatesPage } from "./AgentsTemplatesPage";
import * as agentsApi from "@/api/agents";
import type { CatalogRGD } from "@/types/rgd";

const navigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return { ...actual, useNavigate: () => navigate };
});

vi.mock("@/api/agents");

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AgentsTemplatesPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

function rgd(partial: Partial<CatalogRGD>): CatalogRGD {
  return {
    name: "kagent-rgd-builder-agent",
    namespace: "",
    description: "RGD Builder agent",
    tags: [],
    category: "ai-agents",
    labels: {},
    instances: 0,
    kind: "KnodexAgentTemplate",
    createdAt: "2026-06-10T00:00:00Z",
    updatedAt: "2026-06-10T00:00:00Z",
    ...partial,
  };
}

const list = (items: CatalogRGD[]) => ({ items, totalCount: items.length, page: 1, pageSize: 100 });

describe("AgentsTemplatesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders one row per discovered template", async () => {
    vi.mocked(agentsApi.listAgentTemplates).mockResolvedValue(
      list([rgd({ name: "rgd-builder", instances: 2 }), rgd({ name: "second", description: "Another" })])
    );

    renderPage();

    expect(await screen.findByText("rgd-builder")).toBeInTheDocument();
    expect(screen.getByText("Another")).toBeInTheDocument();
    expect(screen.getAllByTestId("agents-templates-row")).toHaveLength(2);
  });

  it("deploys via the standard /deploy/{name} flow", async () => {
    vi.mocked(agentsApi.listAgentTemplates).mockResolvedValue(list([rgd({ name: "rgd-builder" })]));

    renderPage();

    await screen.findByText("rgd-builder");
    await userEvent.click(screen.getByTestId("deploy-template-button"));

    expect(navigate).toHaveBeenCalledWith("/deploy/rgd-builder");
  });

  it("filters templates by tag", async () => {
    vi.mocked(agentsApi.listAgentTemplates).mockResolvedValue(
      list([
        rgd({ name: "chatbot", tags: ["support", "nlp"] }),
        rgd({ name: "scraper", description: "Web scraper", tags: ["data"] }),
      ])
    );

    renderPage();

    await screen.findByText("chatbot");
    expect(screen.getAllByTestId("agents-templates-row")).toHaveLength(2);

    // Open the tag MultiSelect (role=combobox trigger) and pick "support" —
    // only the chatbot carries it. setup() dispatches the pointer events Radix
    // needs to open the popover under jsdom.
    const user = userEvent.setup();
    const tagTrigger = screen
      .getAllByRole("combobox")
      .find((el) => el.textContent?.includes("Filter by tag..."));
    expect(tagTrigger).toBeDefined();
    await user.click(tagTrigger!);

    // The tag text also renders in the row's Tags column, so scope the option
    // lookup to the popover to avoid an ambiguous match.
    const popover = await waitFor(() => {
      const el = document.querySelector("[data-radix-popper-content-wrapper]");
      if (!el) throw new Error("tag popover not open");
      return el as HTMLElement;
    });
    await user.click(within(popover).getByText("support"));

    expect(screen.getByText("chatbot")).toBeInTheDocument();
    expect(screen.queryByText("scraper")).not.toBeInTheDocument();
  });

  it("shows an empty state explaining how templates are discovered", async () => {
    vi.mocked(agentsApi.listAgentTemplates).mockResolvedValue(list([]));

    renderPage();

    expect(await screen.findByText("No agent templates")).toBeInTheDocument();
    expect(screen.getByText(/kind KnodexAgentTemplate/i)).toBeInTheDocument();
  });

  it("shows a retryable error when the fetch fails", async () => {
    vi.mocked(agentsApi.listAgentTemplates).mockRejectedValueOnce(new Error("network down"));

    renderPage();

    expect(await screen.findByTestId("agents-templates-error")).toBeInTheDocument();
  });
});
