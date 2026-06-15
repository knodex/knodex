// Copyright 2026 Knodex Authors
// SPDX-License-Identifier: AGPL-3.0-only

import { render, screen } from "@testing-library/react";
import { AgentModelBadge } from "./AgentModelBadge";

describe("AgentModelBadge", () => {
  it("renders provider · name when a model is present", () => {
    render(<AgentModelBadge model={{ provider: "OpenAI", name: "gpt-4.1-mini" }} />);
    expect(screen.getByTestId("agent-model-badge")).toHaveTextContent("OpenAI · gpt-4.1-mini");
  });

  it("renders nothing when model is undefined", () => {
    const { container } = render(<AgentModelBadge model={undefined} />);
    expect(screen.queryByTestId("agent-model-badge")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when model is null", () => {
    const { container } = render(<AgentModelBadge model={null} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when provider and name are both empty", () => {
    const { container } = render(<AgentModelBadge model={{ provider: "", name: "" }} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders just the provider when the name is empty", () => {
    render(<AgentModelBadge model={{ provider: "Anthropic", name: "" }} />);
    expect(screen.getByTestId("agent-model-badge")).toHaveTextContent("Anthropic");
  });

  it("renders just the name when the provider is empty", () => {
    render(<AgentModelBadge model={{ provider: "", name: "claude-sonnet-4" }} />);
    expect(screen.getByTestId("agent-model-badge")).toHaveTextContent("claude-sonnet-4");
  });
});
