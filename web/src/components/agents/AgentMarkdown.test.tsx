// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { AgentMarkdown } from "./AgentMarkdown";

describe("AgentMarkdown", () => {
  it("renders a GFM pipe table as a real <table> with header and body cells (the headline fix)", () => {
    const md = [
      "| Category | Notable Kinds |",
      "|---|---|",
      "| Workloads | Deployment |",
      "| Networking | Service |",
    ].join("\n");
    const { container } = render(<AgentMarkdown>{md}</AgentMarkdown>);

    const table = screen.getByRole("table");
    expect(table).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Category" })).toBeInTheDocument();
    expect(within(table).getByRole("columnheader", { name: "Notable Kinds" })).toBeInTheDocument();
    expect(within(table).getByRole("cell", { name: "Deployment" })).toBeInTheDocument();
    expect(within(table).getByRole("cell", { name: "Service" })).toBeInTheDocument();
    // The delimiter row must NOT survive as literal text.
    expect(container.textContent).not.toContain("|---|");
  });

  it("renders headings, bold, italic, lists and inline code as their elements", () => {
    const md = [
      "### Key Resources",
      "",
      "Use **bold** and *italic* and `kubectl`.",
      "",
      "- first",
      "- second",
      "",
      "1. one",
      "2. two",
    ].join("\n");
    render(<AgentMarkdown>{md}</AgentMarkdown>);

    expect(screen.getByRole("heading", { name: "Key Resources" })).toBeInTheDocument();
    expect(screen.getByText("bold").tagName).toBe("STRONG");
    expect(screen.getByText("italic").tagName).toBe("EM");
    expect(screen.getByText("kubectl").tagName).toBe("CODE");
    // One <ul> + one <ol>.
    expect(screen.getAllByRole("list")).toHaveLength(2);
    expect(screen.getByText("first").tagName).toBe("LI");
  });

  it("renders links with target=_blank and rel=noopener noreferrer (AC #5)", () => {
    render(<AgentMarkdown>{"See [the docs](https://knodex.io/docs)."}</AgentMarkdown>);

    const link = screen.getByRole("link", { name: "the docs" });
    expect(link).toHaveAttribute("href", "https://knodex.io/docs");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("renders plain text unchanged (no-markdown regression, AC #6)", () => {
    render(<AgentMarkdown>{"No matching CRDs found for: Redis"}</AgentMarkdown>);
    expect(screen.getByText("No matching CRDs found for: Redis")).toBeInTheDocument();
  });

  // --- AC #3: untrusted-input XSS regression lock ---

  it("does NOT produce an executable element for embedded raw HTML (no <script>/<img>)", () => {
    const { container } = render(
      <AgentMarkdown>
        {"Hello <script>alert(1)</script> and <img src=x onerror=alert(2)>"}
      </AgentMarkdown>
    );
    // No rehype-raw → raw HTML never becomes a live element.
    expect(container.querySelector("script")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    // And no element carries an onerror handler.
    expect(container.querySelector("[onerror]")).toBeNull();
  });

  it("renders a fenced code block as <pre><code> with block styling, not inline chip (pre override)", () => {
    const md = ["```yaml", "apiVersion: v1", "kind: Service", "```"].join("\n");
    const { container } = render(<AgentMarkdown>{md}</AgentMarkdown>);

    const pre = container.querySelector("pre");
    expect(pre).toBeInTheDocument();
    const code = pre?.querySelector("code");
    expect(code).toBeInTheDocument();
    // Block code must NOT carry the inline-chip padding classes.
    expect(code?.className).not.toContain("px-1");
    expect(code?.className).not.toContain("py-0.5");
    // Block code must NOT render as a rounded chip inside the pre.
    expect(code?.className).not.toMatch(/\brounded\b(?!-)/);
  });

  it("neutralizes a javascript: link protocol (no javascript: href, AC #3)", () => {
    const { container } = render(
      <AgentMarkdown>{"[click me](javascript:alert(1))"}</AgentMarkdown>
    );
    const link = container.querySelector("a");
    if (link) {
      expect(link.getAttribute("href") ?? "").not.toContain("javascript:");
    }
    expect(container.innerHTML.toLowerCase()).not.toContain("javascript:");
  });
});
