// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RepositoryList } from "./RepositoryList";
import type { RepositoryConfig } from "@/types/repository";

/**
 * Story 48.7 — connection-registry mental model.
 *
 * These tests pin the render-layer reframing: the Status column reads
 * `Connected`/`Disconnected` (never the raw `valid`/`invalid`/`unknown`), the
 * branch is dropped from the URL cell, a graceful "Connected since" metadatum
 * appears, and the ListFooter counts over the VISIBLE (filtered) rows.
 */

function makeRepo(overrides: Partial<RepositoryConfig> = {}): RepositoryConfig {
  return {
    id: overrides.id ?? "repo-1",
    name: overrides.name ?? "my-repo",
    repoURL: overrides.repoURL ?? "https://github.com/acme/my-repo.git",
    defaultBranch: overrides.defaultBranch ?? "main",
    ...overrides,
  };
}

describe("RepositoryList — connection state badge (AC #2)", () => {
  it("renders Connected (not the raw 'valid') when validationStatus is 'valid'", () => {
    render(<RepositoryList repositories={[makeRepo({ validationStatus: "valid" })]} />);

    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.queryByText("valid")).not.toBeInTheDocument();
    expect(screen.queryByText("Synced")).not.toBeInTheDocument();
    expect(screen.queryByText("Drift")).not.toBeInTheDocument();
  });

  it.each([
    ["invalid" as const],
    ["unknown" as const],
  ])("renders Disconnected when validationStatus is '%s'", (status) => {
    render(<RepositoryList repositories={[makeRepo({ validationStatus: status })]} />);

    expect(screen.getByText("Disconnected")).toBeInTheDocument();
    expect(screen.queryByText(status)).not.toBeInTheDocument();
  });

  it("renders Disconnected when validationStatus is undefined", () => {
    render(<RepositoryList repositories={[makeRepo({ validationStatus: undefined })]} />);

    expect(screen.getByText("Disconnected")).toBeInTheDocument();
    expect(screen.queryByText("Connected")).not.toBeInTheDocument();
  });
});

describe("RepositoryList — URL cell drops the branch (AC #3)", () => {
  it("renders the display URL without the (defaultBranch) suffix", () => {
    render(
      <RepositoryList
        repositories={[
          makeRepo({ repoURL: "https://github.com/acme/my-repo.git", defaultBranch: "release-1.2" }),
        ]}
      />
    );

    expect(screen.getByText("https://github.com/acme/my-repo.git")).toBeInTheDocument();
    // The branch must NOT appear anywhere in the row render.
    expect(screen.queryByText(/release-1\.2/)).not.toBeInTheDocument();
    expect(screen.queryByText(/\(main\)/)).not.toBeInTheDocument();
  });
});

describe("RepositoryList — 'Connected since' metadatum (AC #4)", () => {
  it("shows 'Connected since' when createdAt is present", () => {
    render(
      <RepositoryList
        repositories={[makeRepo({ createdAt: "2026-01-15T10:00:00Z" })]}
      />
    );

    expect(screen.getByText(/Connected since/)).toBeInTheDocument();
  });

  it("omits 'Connected since' (no 'Invalid Date') when createdAt is missing", () => {
    render(<RepositoryList repositories={[makeRepo({ createdAt: undefined })]} />);

    expect(screen.queryByText(/Connected since/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Invalid Date/)).not.toBeInTheDocument();
  });

  it("omits 'Connected since' (no 'Invalid Date') when createdAt is unparseable", () => {
    render(<RepositoryList repositories={[makeRepo({ createdAt: "not-a-date" })]} />);

    expect(screen.queryByText(/Connected since/)).not.toBeInTheDocument();
    expect(screen.queryByText(/Invalid Date/)).not.toBeInTheDocument();
  });
});

describe("RepositoryList — ListFooter (AC #7)", () => {
  const repos = [
    makeRepo({ id: "a", name: "alpha", validationStatus: "valid" }),
    makeRepo({ id: "b", name: "bravo", validationStatus: "invalid" }),
    makeRepo({ id: "c", name: "charlie", validationStatus: "unknown" }),
  ];

  it("summarizes total / connected / disconnected over all rows", () => {
    render(<RepositoryList repositories={repos} />);

    const footer = screen.getByTestId("repositories-list-footer");
    expect(within(footer).getByText("3")).toBeInTheDocument();
    expect(within(footer).getByText("repositories")).toBeInTheDocument();
    // 1 connected (alpha), 2 disconnected (bravo, charlie).
    expect(within(footer).getByText("1")).toBeInTheDocument();
    expect(within(footer).getByText("connected")).toBeInTheDocument();
    expect(within(footer).getByText("2")).toBeInTheDocument();
    expect(within(footer).getByText("disconnected")).toBeInTheDocument();
  });

  it("counts over the VISIBLE (filtered) rows, not the raw set (48.6 pitfall)", async () => {
    const user = userEvent.setup();
    render(<RepositoryList repositories={repos} />);

    await user.type(screen.getByLabelText("Search repositories"), "alpha");

    const footer = screen.getByTestId("repositories-list-footer");
    // Only "alpha" (connected) is visible → counts follow the filtered set, not the
    // raw 3 repos / 1 connected / 2 disconnected.
    expect(footer).toHaveTextContent("1 repositories");
    expect(footer).toHaveTextContent("1 connected");
    expect(footer).toHaveTextContent("0 disconnected");
  });

  it("does not render the footer in the empty state", () => {
    render(<RepositoryList repositories={[]} />);

    expect(screen.queryByTestId("repositories-list-footer")).not.toBeInTheDocument();
    expect(screen.getByText("No repositories yet")).toBeInTheDocument();
  });

  it("does not render the footer in the no-match state", async () => {
    const user = userEvent.setup();
    render(<RepositoryList repositories={repos} />);

    await user.type(screen.getByLabelText("Search repositories"), "zzz-no-match");

    expect(screen.queryByTestId("repositories-list-footer")).not.toBeInTheDocument();
    expect(screen.getByText(/No repositories match/)).toBeInTheDocument();
  });
});

describe("RepositoryList — empty-state copy (AC #8)", () => {
  it("uses connection-registry framing, not GitOps/sync framing", () => {
    render(<RepositoryList repositories={[]} />);

    expect(
      screen.getByText("Connect a Git repository to use it as a deployment source.")
    ).toBeInTheDocument();
    expect(screen.queryByText(/GitOps workflows/)).not.toBeInTheDocument();
    expect(screen.queryByText(/deployment tracking/)).not.toBeInTheDocument();
  });
});

describe("RepositoryList — prototype card layout", () => {
  it("renders a card per repository (no table)", () => {
    render(
      <RepositoryList
        repositories={[
          makeRepo({ id: "r-1", name: "alpha", validationStatus: "valid" }),
          makeRepo({ id: "r-2", name: "bravo", validationStatus: "invalid" }),
        ]}
      />
    );

    const list = screen.getByTestId("repositories-list");
    expect(list.tagName).toBe("UL");
    expect(within(list).getByTestId("repository-card-r-1")).toBeInTheDocument();
    expect(within(list).getByTestId("repository-card-r-2")).toBeInTheDocument();
    // No <table> rendered (the prior layout).
    expect(document.querySelector("table")).toBeNull();
  });

  it("renders the project tag next to the repo name when projectId is set", () => {
    render(
      <RepositoryList
        repositories={[makeRepo({ id: "r-1", name: "alpha", projectId: "payments" })]}
      />
    );

    const card = screen.getByTestId("repository-card-r-1");
    expect(within(card).getByText("alpha")).toBeInTheDocument();
    expect(within(card).getByText("payments")).toBeInTheDocument();
  });

  it("orders Connected repositories before Disconnected ones", () => {
    render(
      <RepositoryList
        repositories={[
          makeRepo({ id: "z-bad", name: "z-disconnected", validationStatus: "invalid" }),
          makeRepo({ id: "a-bad", name: "a-disconnected", validationStatus: undefined }),
          makeRepo({ id: "m-good", name: "m-connected", validationStatus: "valid" }),
          makeRepo({ id: "a-good", name: "a-connected", validationStatus: "valid" }),
        ]}
      />
    );

    const cards = screen.getAllByTestId(/^repository-card-/);
    expect(cards.map((c) => c.getAttribute("data-testid"))).toEqual([
      "repository-card-a-good",   // Connected, name asc
      "repository-card-m-good",   // Connected, name asc
      "repository-card-a-bad",    // Disconnected, name asc
      "repository-card-z-bad",    // Disconnected, name asc
    ]);
  });

  it("activates a card via Enter / Space when onEdit is set", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    const repo = makeRepo({ id: "r-1", name: "alpha" });
    render(<RepositoryList repositories={[repo]} onEdit={onEdit} />);

    const card = screen.getByTestId("repository-card-r-1");
    expect(card).toHaveAttribute("role", "button");
    expect(card).toHaveAttribute("tabindex", "0");

    card.focus();
    await user.keyboard("{Enter}");
    expect(onEdit).toHaveBeenCalledWith(repo);

    onEdit.mockClear();
    await user.keyboard(" ");
    expect(onEdit).toHaveBeenCalledWith(repo);
  });
});
