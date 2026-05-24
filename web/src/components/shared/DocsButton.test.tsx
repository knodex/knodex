// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { DocsButton } from "./DocsButton";

describe("DocsButton", () => {
  it("renders a link with the docs URL when provided", () => {
    render(<DocsButton docsUrl="https://docs.example.com/webapp" rgdLabel="My WebApp" />);
    const link = screen.getByRole("link", { name: /open documentation for my webapp/i });
    expect(link).toHaveAttribute("href", "https://docs.example.com/webapp");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(link).toHaveTextContent("Docs");
  });

  it("uses a custom label when provided", () => {
    render(
      <DocsButton
        docsUrl="https://docs.example.com/webapp"
        rgdLabel="My WebApp"
        label="View documentation"
      />
    );
    expect(screen.getByRole("link")).toHaveTextContent("View documentation");
  });

  it("renders nothing when docsUrl is absent", () => {
    const { container } = render(<DocsButton rgdLabel="My WebApp" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when docsUrl is an empty string", () => {
    const { container } = render(<DocsButton docsUrl="" rgdLabel="My WebApp" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when docsUrl uses an unsafe scheme", () => {
    const { container } = render(
      <DocsButton docsUrl="javascript:alert(1)" rgdLabel="My WebApp" />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when docsUrl is malformed", () => {
    const { container } = render(<DocsButton docsUrl="not a url" rgdLabel="My WebApp" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing for data: URLs", () => {
    const { container } = render(
      <DocsButton docsUrl="data:text/html,<script>alert(1)</script>" rgdLabel="X" />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing for blob: URLs", () => {
    const { container } = render(
      <DocsButton docsUrl="blob:https://trusted.com/abc" rgdLabel="X" />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing for file: URLs", () => {
    const { container } = render(<DocsButton docsUrl="file:///etc/passwd" rgdLabel="X" />);
    expect(container).toBeEmptyDOMElement();
  });

  it("accepts plain http URLs (not just https)", () => {
    render(<DocsButton docsUrl="http://internal.docs/page" rgdLabel="X" />);
    expect(screen.getByRole("link")).toHaveAttribute("href", "http://internal.docs/page");
  });
});
