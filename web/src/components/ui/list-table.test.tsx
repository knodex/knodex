// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render } from "@testing-library/react";
import {
  ListTableShell,
  ListTableHeader,
  tableHeaderClasses,
  tableHeaderStickyClasses,
  tableShellClasses,
} from "./list-table";

describe("ListTableShell", () => {
  it("renders the rounded card shell with default animation", () => {
    const { container } = render(<ListTableShell />);
    const div = container.firstChild as HTMLElement;
    expect(div.className).toMatch(/rounded-lg/);
    expect(div.className).toMatch(/border/);
    expect(div.className).toMatch(/animate-fade-in-up/);
  });

  it("suppresses the entrance animation when noAnimation is set", () => {
    const { container } = render(<ListTableShell noAnimation />);
    const div = container.firstChild as HTMLElement;
    expect(div.className).not.toMatch(/animate-fade-in-up/);
  });

  it("merges consumer className with internal styling", () => {
    const { container } = render(<ListTableShell className="custom-host" />);
    const div = container.firstChild as HTMLElement;
    expect(div.className).toMatch(/custom-host/);
    expect(div.className).toMatch(/rounded-lg/);
  });
});

describe("ListTableHeader", () => {
  it("renders a <thead> with the shared bg styling", () => {
    const { container } = render(
      <table>
        <ListTableHeader data-testid="thead" />
      </table>
    );
    const thead = container.querySelector("thead");
    expect(thead).not.toBeNull();
    expect(thead!.className).toMatch(/bg-card/);
  });

  it("adds sticky positioning when sticky is set", () => {
    const { container } = render(
      <table>
        <ListTableHeader sticky />
      </table>
    );
    const thead = container.querySelector("thead")!;
    expect(thead.className).toMatch(/sticky/);
    expect(thead.className).toMatch(/top-\[52px\]/);
  });

  it("omits sticky positioning by default", () => {
    const { container } = render(
      <table>
        <ListTableHeader />
      </table>
    );
    expect(container.querySelector("thead")!.className).not.toMatch(/sticky/);
  });
});

describe("class helpers", () => {
  it("expose stable strings for inline consumers", () => {
    expect(tableShellClasses).toMatch(/rounded-lg/);
    expect(tableHeaderClasses).toMatch(/bg-card/);
    expect(tableHeaderStickyClasses).toMatch(/sticky/);
  });
});
