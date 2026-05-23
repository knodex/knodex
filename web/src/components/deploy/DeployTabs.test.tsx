// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { useEffect } from "react";
import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { FormProvider, useForm } from "react-hook-form";
import { DeployTabs } from "./DeployTabs";
import type { DeployTab } from "@/lib/build-tabs";

interface HarnessProps {
  tabs: DeployTab[];
  activeId: string;
  visitedIds: Set<string>;
  errors: Record<string, { message?: string }>;
  onSelect?: (id: string) => void;
}

function Harness({
  tabs,
  activeId,
  visitedIds,
  errors,
  onSelect,
}: HarnessProps) {
  const methods = useForm();
  useEffect(() => {
    for (const [field, err] of Object.entries(errors)) {
      methods.setError(field, { type: "manual", message: err.message });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <FormProvider {...methods}>
      <DeployTabs
        tabs={tabs}
        activeId={activeId}
        onSelect={onSelect ?? (() => undefined)}
        visitedIds={visitedIds}
      />
    </FormProvider>
  );
}

const SAMPLE_TABS: DeployTab[] = [
  {
    id: "general",
    kind: "general",
    label: "General",
    properties: { replicas: { type: "integer" } },
  },
  {
    id: "networking",
    kind: "schema",
    label: "Networking",
    properties: { port: { type: "integer" } },
  },
  { id: "review", kind: "review", label: "Review + Deploy" },
];

function badgeState(tabId: string): string | null {
  const badge = screen.getByTestId(`deploy-tab-badge-${tabId}`);
  return badge.getAttribute("data-state");
}

describe("DeployTabs", () => {
  it("renders gray badges when no errors and no visits", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="general"
        visitedIds={new Set()}
        errors={{}}
      />
    );
    expect(badgeState("general")).toBe("untouched");
    expect(badgeState("networking")).toBe("untouched");
    expect(badgeState("review")).toBe("untouched");
  });

  it("lights the General badge red when instanceName has an error (Knodex plumbing owned by General)", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="general"
        visitedIds={new Set(["general"])}
        errors={{ instanceName: { message: "required" } }}
      />
    );
    expect(badgeState("general")).toBe("error");
  });

  it("lights the General badge red when a top-level schema scalar has an error", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="general"
        visitedIds={new Set(["general"])}
        errors={{ replicas: { message: "required" } }}
      />
    );
    expect(badgeState("general")).toBe("error");
  });

  it("error overrides green on visited tabs", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="networking"
        visitedIds={new Set(["general", "networking"])}
        errors={{ networking: { message: "port required" } }}
      />
    );
    expect(badgeState("networking")).toBe("error");
  });

  it("shows green check on visited tabs with no owned errors", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="general"
        visitedIds={new Set(["general"])}
        errors={{}}
      />
    );
    expect(badgeState("general")).toBe("valid");
    expect(badgeState("networking")).toBe("untouched");
  });

  it("review badge is never red", () => {
    render(
      <Harness
        tabs={SAMPLE_TABS}
        activeId="review"
        visitedIds={new Set(["general", "review"])}
        errors={{
          instanceName: { message: "required" },
          networking: { message: "port required" },
        }}
      />
    );
    expect(badgeState("review")).not.toBe("error");
  });
});
